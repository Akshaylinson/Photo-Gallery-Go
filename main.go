package main

import (
    "database/sql"
    "encoding/json"
    "fmt"
    "html/template"
    "io"
    "log"
    "net/http"
    "os"
    "path/filepath"
    "strconv"
    "strings"
    "time"

    _ "modernc.org/sqlite"

    "github.com/disintegration/imaging"
    "github.com/gorilla/mux"
    "github.com/google/uuid"
)


const (
	imagesDir     = "images"
	thumbsDir     = "thumbs"
	dbFile        = "gallery.db"
	maxUploadSize = 20 << 20 // 20 MB
	defaultPer    = 12
)

var templates *template.Template
var db *sql.DB

type ImageRow struct {
	ID        string
	Filename  string
	Title     string
	Album     string
	CreatedAt time.Time
}

func main() {
	ensureDirs()
	loadTemplates()
	openDB()

	r := mux.NewRouter()
	// static file servers
	r.PathPrefix("/images/").Handler(http.StripPrefix("/images/", http.FileServer(http.Dir(imagesDir))))
	r.PathPrefix("/thumbs/").Handler(http.StripPrefix("/thumbs/", http.FileServer(http.Dir(thumbsDir))))

	// routes
	r.HandleFunc("/", galleryHandler).Methods("GET")
	r.HandleFunc("/upload", uploadHandler).Methods("POST")
	r.HandleFunc("/thumb/{size}/{filename}", thumbHandler).Methods("GET")
	r.HandleFunc("/api/images", apiImagesHandler).Methods("GET")

	addr := ":8080"
	log.Printf("starting server on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatal(err)
	}
}

func ensureDirs() {
	for _, d := range []string{imagesDir, thumbsDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			log.Fatalf("create dir %s: %v", d, err)
		}
	}
}

func loadTemplates() {
	var err error
	templates, err = template.ParseGlob("templates/*.html")
	if err != nil {
		log.Fatalf("parse templates: %v", err)
	}
}

func openDB() {
	var err error
	db, err = sql.Open("sqlite3", dbFile)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	create := `
	CREATE TABLE IF NOT EXISTS images (
	  id TEXT PRIMARY KEY,
	  filename TEXT NOT NULL,
	  title TEXT,
	  album TEXT,
	  created_at INTEGER NOT NULL
	);
	`
	if _, err := db.Exec(create); err != nil {
		log.Fatalf("create table: %v", err)
	}
}

func galleryHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := atoiDefault(q.Get("page"), 1)
	per := atoiDefault(q.Get("per"), defaultPer)
	album := q.Get("album")
	offset := (page - 1) * per

	var rows *sql.Rows
	var err error
	if album == "" {
		rows, err = db.Query("SELECT id, filename, title, album, created_at FROM images ORDER BY created_at DESC LIMIT ? OFFSET ?", per, offset)
	} else {
		rows, err = db.Query("SELECT id, filename, title, album, created_at FROM images WHERE album = ? ORDER BY created_at DESC LIMIT ? OFFSET ?", album, per, offset)
	}
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	defer rows.Close()

	images := []ImageRow{}
	for rows.Next() {
		var id, filename, title, alb string
		var createdAt int64
		if err := rows.Scan(&id, &filename, &title, &alb, &createdAt); err != nil {
			continue
		}
		images = append(images, ImageRow{
			ID:        id,
			Filename:  filename,
			Title:     title,
			Album:     alb,
			CreatedAt: time.Unix(createdAt, 0),
		})
	}

	// total count for pagination
	var total int
	if album == "" {
		_ = db.QueryRow("SELECT COUNT(1) FROM images").Scan(&total)
	} else {
		_ = db.QueryRow("SELECT COUNT(1) FROM images WHERE album = ?", album).Scan(&total)
	}

	data := map[string]interface{}{
		"Images": images,
		"Page":   page,
		"Per":    per,
		"Total":  total,
		"Album":  album,
	}
	if err := templates.ExecuteTemplate(w, "index.html", data); err != nil {
		http.Error(w, err.Error(), 500)
	}
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(maxUploadSize)
	if err != nil {
		http.Error(w, "file too big or invalid form", http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "image required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	title := r.FormValue("title")
	album := r.FormValue("album")

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext == "" {
		ext = ".jpg"
	}
	id := uuid.New().String()
	filename := id + ext
	outPath := filepath.Join(imagesDir, filename)

	out, err := os.Create(outPath)
	if err != nil {
		http.Error(w, "unable to save file", 500)
		return
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		http.Error(w, "save error", 500)
		return
	}

	_, err = db.Exec("INSERT INTO images(id, filename, title, album, created_at) VALUES(?,?,?,?,?)", id, filename, title, album, time.Now().Unix())
	if err != nil {
		log.Println("db insert error:", err)
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func thumbHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	size := vars["size"]
	filename := filepath.Base(vars["filename"])

	parts := strings.Split(size, "x")
	if len(parts) != 2 {
		http.Error(w, "invalid size", 400)
		return
	}
	wid, err1 := strconv.Atoi(parts[0])
	hei, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || wid <= 0 || hei <= 0 {
		http.Error(w, "invalid size numbers", 400)
		return
	}

	thumbName := fmt.Sprintf("%dx%d_%s", wid, hei, filename)
	thumbPath := filepath.Join(thumbsDir, thumbName)
	if _, err := os.Stat(thumbPath); err == nil {
		serveFileWithCache(w, r, thumbPath)
		return
	}

	srcPath := filepath.Join(imagesDir, filename)
	if _, err := os.Stat(srcPath); err != nil {
		http.NotFound(w, r)
		return
	}

	img, err := imaging.Open(srcPath)
	if err != nil {
		http.Error(w, "open image failed", 500)
		return
	}
	thumb := imaging.Fit(img, wid, hei, imaging.Lanczos)

	if err := imaging.Save(thumb, thumbPath); err != nil {
		http.Error(w, "save thumb failed", 500)
		return
	}

	serveFileWithCache(w, r, thumbPath)
}

func serveFileWithCache(w http.ResponseWriter, r *http.Request, path string) {
	stat, err := os.Stat(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	mod := stat.ModTime().UTC().Format(http.TimeFormat)
	etag := fmt.Sprintf(`W/"%d-%d"`, stat.Size(), stat.ModTime().Unix())
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("Last-Modified", mod)
	w.Header().Set("ETag", etag)

	if match := r.Header.Get("If-None-Match"); match != "" {
		if strings.Contains(match, etag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	if ims := r.Header.Get("If-Modified-Since"); ims != "" {
		if t, err := time.Parse(http.TimeFormat, ims); err == nil {
			if stat.ModTime().Before(t.Add(1 * time.Second)) {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}
	}

	http.ServeFile(w, r, path)
}

func apiImagesHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := atoiDefault(q.Get("page"), 1)
	per := atoiDefault(q.Get("per"), defaultPer)
	album := q.Get("album")
	offset := (page - 1) * per

	var rows *sql.Rows
	var err error
	if album == "" {
		rows, err = db.Query("SELECT id, filename, title, album, created_at FROM images ORDER BY created_at DESC LIMIT ? OFFSET ?", per, offset)
	} else {
		rows, err = db.Query("SELECT id, filename, title, album, created_at FROM images WHERE album = ? ORDER BY created_at DESC LIMIT ? OFFSET ?", album, per, offset)
	}
	if err != nil {
		http.Error(w, "db err", 500)
		return
	}
	defer rows.Close()
	images := []ImageRow{}
	for rows.Next() {
		var id, filename, title, alb string
		var createdAt int64
		if err := rows.Scan(&id, &filename, &title, &alb, &createdAt); err != nil {
			continue
		}
		images = append(images, ImageRow{
			ID:        id,
			Filename:  filename,
			Title:     title,
			Album:     alb,
			CreatedAt: time.Unix(createdAt, 0),
		})
	}
	type resp struct {
		Page   int        `json:"page"`
		Per    int        `json:"per"`
		Images []ImageRow `json:"images"`
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp{Page: page, Per: per, Images: images})
}

func atoiDefault(s string, d int) int {
	if s == "" {
		return d
	}
	i, err := strconv.Atoi(s)
	if err != nil || i <= 0 {
		return d
	}
	return i
}

