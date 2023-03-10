package localbooru

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "embed"
	_ "github.com/mattn/go-sqlite3"
)

//go:embed schema.sql
var dbSchema string

type database struct {
	c *sql.DB
}

type Post struct {
	ID int64 `json:"id"`

	Author string   `json:"author,omitempty"` // Dunno if correct
	Score  int      `json:"score"`
	Source string   `json:"source,omitempty"`
	Rating string   `json:"rating"`
	Tags   []string `json:"-"`

	Created time.Time `json:"created_at"`
	Updated time.Time `json:"updated_at"`

	// Booru is the source booru.
	// Populated by Boorumux. Not in Danbooru API.
	Booru string `json:"booru,omitempty"`

	// Booru is the ID of this post from the source booru.
	// Populated by Boorumux. Not in Danbooru API.
	BooruID string `json:"booru_id,omitempty"`

	Hash   string `json:"md5"`
	Ext    string `json:"file_ext"`
	Width  int    `json:"image_width"`
	Height int    `json:"image_height"`

	// TagString is Tags, but concatenated to conform to Danbooru's API.
	//
	// This field is not populated by any database methods and exists
	// solely to aid JSON marshalling.
	TagString string `json:"tag_string"`

	// FileUrl is the URL to the full sized version of this post.
	//
	// This field is not populated by any database methods and exists
	// solely to aid JSON marshalling.
	FileUrl string `json:"file_url"`

	// ThumbUrl is the URL to the "large" version of this post.
	// TODO: I forget what the thumbnail version is, so this is the
	// thumbnail size.
	//
	// This field is not populated by any database methods and exists
	// solely to aid JSON marshalling.
	ThumbUrl string `json:"large_file_url,omitempty"`
}

var dbUpgrades = []string{
	"", // blank to count for schema
	`
		CREATE TABLE posts_new(
			id INTEGER PRIMARY KEY,

			author TEXT,
			score INTEGER NOT NULL DEFAULT 0,
			source TEXT,
			rating TEXT,
			created INTEGER NOT NULL DEFAULT 0,
			updated INTEGER NOT NULL DEFAULT 0,

			booru TEXT,
			booru_id TEXT,

			hash TEXT UNIQUE NOT NULL,
			ext TEXT NOT NULL,
			width INTEGER NOT NULL DEFAULT 0,
			height INTEGER NOT NULL DEFAULT 0
		);
		INSERT INTO posts_new SELECT * FROM posts;
		DROP TABLE posts;
		ALTER TABLE posts_new RENAME TO posts;
	`,
}

func opendb(path string) (*database, error) {
	c, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	// Exec schema if we need to
	ver := 0
	if err := c.QueryRow("PRAGMA user_version").Scan(&ver); err != nil {
		c.Close()
		return nil, err
	}

	if ver < len(dbUpgrades) {
		if ver == 0 {
			if _, err := c.Exec(dbSchema); err != nil {
				c.Close()
				return nil, err
			}
		} else {
			for _, v := range dbUpgrades[ver:] {
				if _, err := c.Exec(v); err != nil {
					c.Close()
					return nil, err
				}
			}
		}

		if _, err := c.Exec("PRAGMA user_version=" + fmt.Sprint(len(dbUpgrades))); err != nil {
			c.Close()
			return nil, err
		}
	}

	return &database{c: c}, nil
}

// Post returns a post's information for its id.
func (d *database) Post(ctx context.Context, id int64) (Post, error) {
	p := Post{ID: id}

	tx, err := d.c.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return p, err
	}
	defer tx.Rollback()

	var ct, ut int64

	if err := tx.QueryRowContext(ctx, `SELECT author, score, source, rating, created, updated, booru, booru_id, hash, ext, width, height FROM posts WHERE id = ?`, id).Scan(&p.Author, &p.Score, &p.Source, &p.Rating, &ct, &ut, &p.Booru, &p.BooruID, &p.Hash, &p.Ext, &p.Width, &p.Height); err != nil {
		return p, err
	}

	p.Created = time.Unix(ct, 0)
	p.Updated = time.Unix(ut, 0)

	rows, err := tx.QueryContext(ctx, `SELECT tag FROM posttag WHERE post = ?`, id)
	if err != nil {
		return p, err
	}
	defer rows.Close()

	for rows.Next() {
		s := ""
		if err := rows.Scan(&s); err != nil {
			return p, err
		}
		p.Tags = append(p.Tags, s)
	}

	return p, tx.Commit()
}

func makePostQuery(query []string, offset, limit int) (string, []interface{}) {
	// I have no idea on how best to do this
	// I also absolutely hate this function

	s := strings.Builder{}
	a := []interface{}{}
	id := false
	in := ""
	rating := ""

	for _, v := range query {
		if strings.HasPrefix(v, "rating:") {
			rating = strings.TrimPrefix(v, "rating:")
			continue
		} else if strings.HasPrefix(v, "score:") {
			// TODO
			continue
		} else if strings.HasPrefix(v, "md5:") {
			// This is special in that it will only match one file or none at all
			// Therefore we can just do one query instead of matching tags
			return `SELECT id, author, score, source, rating, created, updated, booru, booru_id, hash, ext, width, height FROM posts WHERE hash = ?`, []interface{}{strings.TrimPrefix(v, "md5:")}
		}

		// TODO: This is wrong
		// It incorrectly matches against any tags instead of ensuring that all exist
		if !id {
			s.WriteString(`SELECT post FROM posttag GROUP BY post HAVING SUM(tag in (`)
		} else {
			s.WriteString(`,`)
		}
		id = true
		s.WriteString(`?`)
		a = append(a, v)
	}

	if id {
		s.WriteString(`)) = ?`)

		// len(a) = the amount of tags we're looking for
		a = append(a, len(a))

		in = s.String()
		s.Reset()
	}

	s.WriteString(`SELECT id, author, score, source, rating, created, updated, booru, booru_id, hash, ext, width, height FROM posts`)

	if in != "" {
		s.WriteString(" WHERE id IN (")
		s.WriteString(in)
		s.WriteRune(')')
	}

	if rating != "" {
		if in != "" {
			s.WriteString(" AND")
		} else {
			s.WriteString(" WHERE")
		}

		s.WriteString(" rating = ?")
		a = append(a, rating)
	}

	s.WriteString(" ORDER BY id DESC")

	if limit > 0 {
		s.WriteString(" LIMIT ?")
		a = append(a, limit)
	}

	if offset > 0 {
		s.WriteString(" OFFSET ?")
		a = append(a, offset)
	}

	return s.String(), a
}

// Posts returns a collection of posts which match the search terms provided.
//
// If offset or limit is provided, the response will be offset or limited to an amount.
// Use offset or limit to provide pagination.
// There is no way to find out the total number of posts, but when the length
// of the returned posts doesn't equal limit and limit is not equal to zero,
// there are no more posts.
func (d *database) Posts(ctx context.Context, search []string, offset, limit int) ([]Post, error) {
	tx, err := d.c.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	query, args := makePostQuery(search, offset, limit)
	fmt.Println(query)
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	if limit == 0 {
		limit = 20
	}

	posts := make([]Post, 0, limit)

	for rows.Next() {
		p := Post{}
		var ct, ut int64

		if err := rows.Scan(&p.ID, &p.Author, &p.Score, &p.Source, &p.Rating, &ct, &ut, &p.Booru, &p.BooruID, &p.Hash, &p.Ext, &p.Width, &p.Height); err != nil {
			rows.Close()
			return posts, err
		}

		p.Created = time.Unix(ct, 0)
		p.Updated = time.Unix(ut, 0)

		posts = append(posts, p)
	}
	rows.Close()

	for i, p := range posts {
		rows, err := tx.QueryContext(ctx, `SELECT tag FROM posttag WHERE post = ?`, p.ID)
		if err != nil {
			return posts, err
		}

		for rows.Next() {
			s := ""
			if err := rows.Scan(&s); err != nil {
				rows.Close()
				return posts, err
			}
			p.Tags = append(p.Tags, s)
		}
		rows.Close()

		posts[i] = p
	}

	return posts, tx.Commit()
}

// SavePost writes a post's information to the database.
// If successful, p.ID will be written with the post's ID.
func (d *database) SavePost(ctx context.Context, p *Post) error {
	tx, err := d.c.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Save post info first to get post ID
	n, err := tx.ExecContext(ctx, `INSERT INTO posts(author, score, source, rating, created, updated, booru, booru_id, hash, ext, width, height) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, p.Author, p.Score, p.Source, p.Rating, p.Created.UTC().Unix(), p.Updated.UTC().Unix(), p.Booru, p.BooruID, p.Hash, p.Ext, p.Width, p.Height)
	if err != nil {
		return err
	}

	id, err := n.LastInsertId()
	if err != nil {
		return err
	}
	// We don't save the ID yet because it might fail

	// Save tags
	for _, v := range p.Tags {
		_, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO posttag(post, tag) VALUES(?, ?)`, id, v)
		if err != nil {
			return err
		}
	}

	p.ID = id
	return tx.Commit()
}

func (d *database) Delete(ctx context.Context, p int64) error {
	tx, err := d.c.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete from posts table
	_, err = tx.ExecContext(ctx, `DELETE FROM posts WHERE id = ?`, p)
	if err != nil {
		return err
	}

	// Delete from posttag table
	_, err = tx.ExecContext(ctx, `DELETE FROM posttag WHERE post = ?`, p)
	if err != nil {
		return err
	}

	return tx.Commit()
}
