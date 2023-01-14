package localbooru

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
)

const pageLimit = 50

// HTTP implements the net/http handler for Localbooru's API.
type HTTP struct {
	BaseURL string

	db *database
	fs http.Handler
}

func binHash(h string) string {
	return h[:2] + "/" + h[2:4]
}

func (h *HTTP) xfrm(v Post) Post {
	v.FileUrl = fmt.Sprintf("%s/img/%s/%s.%s", h.BaseURL, binHash(v.Hash), v.Hash, v.Ext)
	v.ThumbUrl = fmt.Sprintf("%s/img/%s/%s.thumb.%s", h.BaseURL, binHash(v.Hash), v.Hash, v.Ext)
	v.TagString = strings.Join(v.Tags, " ")

	return v
}

func (h *HTTP) xfrms(p []Post) {
	for i, v := range p {
		p[i] = h.xfrm(v)
	}
}

func (h *HTTP) posts(w http.ResponseWriter, r *http.Request) error {
	qv := r.URL.Query()

	page := 0
	if p := qv.Get("page"); p != "" {
		page, _ = strconv.Atoi(p)
	}
	offset := page * pageLimit

	query := []string(nil)
	if q := qv.Get("tags"); q != "" {
		query = strings.Split(q, " ")
	}

	posts, err := h.db.Posts(r.Context(), query, offset, pageLimit)
	if err != nil {
		return err
	}

	h.xfrms(posts)
	return json.NewEncoder(w).Encode(posts)
}

func (h *HTTP) post(w http.ResponseWriter, r *http.Request) error {
	sid := strings.TrimSuffix(strings.TrimPrefix(r.URL.EscapedPath(), "/posts/"), ".json")
	id, err := strconv.ParseInt(sid, 10, 64)
	if err != nil {
		return err
	}

	post, err := h.db.Post(r.Context(), id)
	if err != nil {
		return err
	}

	post = h.xfrm(post)
	return json.NewEncoder(w).Encode(post)
}

func (h *HTTP) newPost(w http.ResponseWriter, r *http.Request) error {
	// TODO: Drastically rewrite this.
	// All of this was brought together in a few hours in one night with
	// one goal: to get this to work.

	if r.Method != "POST" {
		panic("not POST")
	}

	mr, err := r.MultipartReader()
	if err != nil {
		return err
	}

	var pi *Post
	var tf *os.File
	var ext string
	md := md5.New()

	for p, err := mr.NextPart(); err == nil; p, err = mr.NextPart() {
		if p.FormName() == "info" {
			if err := json.NewDecoder(p).Decode(&pi); err != nil {
				return err
			}
			fmt.Println("read post info")
		} else if p.FormName() == "file" {
			if tf != nil {
				panic("too many files")
			}

			ext = path.Ext(p.FileName())[1:]

			fmt.Println("read file...")

			// Write to temp file
			tf, err = os.CreateTemp("", "lbupload*."+ext)
			if err != nil {
				return err
			}
			defer os.Remove(tf.Name())
			defer tf.Close()

			if _, err := io.Copy(tf, io.TeeReader(p, md)); err != nil {
				return err
			}
			fmt.Println("save file")
		}
	}

	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}

	if pi == nil || tf == nil {
		// TODO
		panic("supply pi or tf")
	}

	fmt.Println(pi.TagString)

	// Populate some info
	pi.Tags = strings.Split(pi.TagString, " ")
	pi.Hash = hex.EncodeToString(md.Sum(nil))
	pi.Ext = ext

	// TODO: width, height

	// Write to database
	if err := h.db.SavePost(r.Context(), pi); err != nil {
		return err
	}

	// Write to img
	pth := fmt.Sprintf("./img/%s/%s.%s", binHash(pi.Hash), pi.Hash, pi.Ext)
	dir := path.Dir(pth)
	if err := os.MkdirAll(dir, 0777); err != nil {
		return err
	}

	return os.Rename(tf.Name(), pth)
}

func (h *HTTP) Open(path string) error {
	var err error
	h.db, err = opendb(path)
	return err
}

func (h *HTTP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := r.URL.EscapedPath()

	var err error
	if p == "/posts.json" {
		err = h.posts(w, r)
	} else if p == "/post" {
		err = h.newPost(w, r)
	} else if strings.HasPrefix(p, "/posts/") {
		err = h.post(w, r)
	} else if strings.HasPrefix(p, "/img/") {
		if h.fs == nil {
			h.fs = http.StripPrefix("/img", http.FileServer(http.Dir("./img")))
		}

		h.fs.ServeHTTP(w, r)
		return
	}

	if err != nil {
		panic(err)
	}
}
