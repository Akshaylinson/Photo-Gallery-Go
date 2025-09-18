 📸 Photo Gallery in Go

A simple **photo gallery web app** built with Go.  
Features include file uploads, automatic thumbnail generation, image serving with caching headers, and album-style browsing.  
Uses **Bootstrap** for styling and **SQLite** for metadata storage.

---

## 🚀 Features

- Upload photos from the browser
- Automatic thumbnail generation (using `imaging`)
- Store metadata in **SQLite**
- Serve images and thumbnails with proper caching headers
- Clean Bootstrap UI (single-page app)
- Pagination support for large galleries
- Extendable: add albums, S3-compatible storage, or authentication

---

## 🛠 Tech Stack

- **Go 1.25+**
- [gorilla/mux](https://github.com/gorilla/mux) – HTTP router
- [disintegration/imaging](https://github.com/disintegration/imaging) – Image processing
- [google/uuid](https://github.com/google/uuid) – Unique IDs for uploads
- [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) – Pure Go SQLite driver (no C toolchain required)
- **Bootstrap 5** via CDN

---

## 📂 Project Structure

photo-gallery-go/
│── images/ # uploaded photos
│── thumbs/ # generated thumbnails
│── templates/
│ └── index.html # main UI template
│── gallery.db # SQLite database
│── main.go # Go server
│── go.mod # Go module file
│── go.sum # Dependency checksums
│── README.md # This file
│── run.ps1 # Windows helper script
│── run.sh # Linux/macOS helper script

yaml
Copy code

---

## ⚡️ Setup & Run

### 1. Clone the repo
```bash
git clone https://github.com/you/photo-gallery-go.git
cd photo-gallery-go
2. Install dependencies
powershell
Copy code
go mod tidy
3. Run the server
powershell
Copy code
go run main.go
4. Open in browser
Go to http://localhost:8080

🔧 Windows Notes
Uses modernc.org/sqlite (pure Go). No external C compiler is required.

If you prefer mattn/go-sqlite3, you’ll need to install a C toolchain (e.g., MinGW).

📦 Future Enhancements
Add albums with subfolders

User authentication

Store images on Amazon S3, MinIO, or GCP storage

Image search and tags

