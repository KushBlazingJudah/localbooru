package localbooru

import (
	"database/sql"
	"context"
	"time"
	"strings"
)

type database struct {
	c *sql.Conn 
}

type Post struct {
	ID int64

	Author string
	Score int
	Source string
	Rating string
	Tags []string

	Created time.Time
	Updated time.Time

	Booru string
	BooruID string

	Hash string
	Ext string
	Width int
	Height int
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

	err = tx.QueryRowContext(ctx, `SELECT author, score, source, rating, created, updated, booru, booru_id, hash, ext, width, height FROM posts WHERE id = ?`, p).Scan(&p.Author, &p.Score, &p.Source, &p.Rating, &ct, &ut, &p.Booru, &p.BooruID, &p.Hash, &p.Ext, &p.Width, &p.Height)

	p.Created = time.Unix(ct, 0)
	p.Updated = time.Unix(ut, 0)

	rows, err := tx.QueryContext(ctx, `SELECT tag FROM posttag WHERE id = ?`, id)
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
	id := len(query) > 0
	in := ""
	rating := ""

	if id {
		s.WriteString(`SELECT id FROM posttag WHERE`)
		for i, v := range query {
			if strings.HasPrefix(v, "rating:") {
				rating = strings.TrimPrefix(v, "rating:")
				continue
			} else if strings.HasPrefix(v, "score:") {
				// TODO
				continue
			}

			if i > 0 {
				s.WriteString(` OR`)
			}
			s.WriteString( `tag = ?`)
			a = append(a, v)
		}

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
			s.WriteString(" OR")
		} else {
			s.WriteString(" WHERE")
		}

		s.WriteString(" rating = ?")
		a = append(a, rating)
	}

	if offset > 0 {
		s.WriteString(" OFFSET ?")
		a = append(a, offset)
	}

	if limit > 0 {
		s.WriteString(" LIMIT ?")
		a = append(a, limit)
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
	}
	rows.Close()

	for i, p := range posts {
		rows, err := tx.QueryContext(ctx, `SELECT tag FROM posttag WHERE id = ?`, p.ID)
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
		_, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO posttag(post, tag) VALUES(?, ?)`, p.ID, v)
		if err != nil {
			return err
		}
	}

	p.ID = id
	return tx.Commit()
}
