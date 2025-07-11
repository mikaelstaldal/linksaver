# Link Saver

A simple and efficient web application for saving and managing your favorite links. 
Link Saver automatically extracts page title, description and optionally screenshots from URLs and 
provides a web interface for organizing your bookmarks.

## Features

- **Save Links**: Add URLs with automatic title and description extraction and screenshots from web pages
- **SQLite Storage**: Lightweight, file-based database with no external dependencies
- **Docker Support**: Easy deployment with Docker containers
- **HTTP basic auth**: Protect the application with username and password

## Run with Docker (with screenshot support)

1. Build the image:
   ```bash
   docker build -t linksaver .
   ```
2. Run the container without authentication:
   ```bash
   docker run --mount "type=bind,src=$(pwd)/data,dst=/data" --cap-drop ALL --security-opt no-new-privileges -p 127.0.0.1:8080:8080 linksaver
   ```
3. Run the container with HTTP basic authentication:
   ```bash
   docker run --mount "type=bind,src=$(pwd)/data,dst=/data" --cap-drop ALL --security-opt no-new-privileges -p 127.0.0.1:8080:8080 -e BASIC_AUTH=$(htpasswd -nBC 12 my_username) linksaver
   ```
Note: This is only secure if you also use https.   

The application will store data in the directory mounted at `/data`, use `data/linksaver.sqlite` as the database file 
and store screenshots in `data/screenshots`. 


## Run without Docker (without screenshot support)

1. Build the standalone executable
   ```bash
   go build -v ./cmd/linksaver/
   ```
2. Run it without authentication:
   ```bash
   ./linksaver -port 8080
   ```  
3. Run it with HTTP basic authentication:
   ```bash
   BASIC_AUTH=$(htpasswd -nBC 12 my_username) ./linksaver -port 8080
   ```  
Note: This is only secure if you also use https.   

The application will store data in the current directory, use `./linksaver.sqlite` as the database file.


## Web Interface

Once the application is running, open your web browser and navigate to:
- `http://localhost:8080` (or your configured port)

From the web interface, you can:
1. **Add a new link**: Enter a URL in the input field and click "Add Link"
2. **View all links**: All saved links are displayed on the main page
3. **Delete a link**: Click the delete button next to any link to remove it

## Development

### Project Structure

```
├── cmd/linksaver/          # Main application
│   ├── main.go             # Application entry point
│   ├── handlers.go         # HTTP request handlers
│   ├── handlers_test.go    # Handler tests
│   └── db/                 # Database layer
│       ├── db.go           # Database operations
│       └── db_test.go      # Database tests
├── ui/                     # User interface assets
│   ├── templates/          # HTML templates
│   └── static/             # CSS, JavaScript files
├── Dockerfile              # Docker configuration
├── run.sh                  # Start script for Docker image
├── go.mod                  # Go module definition
└── README.md               # This file
```

### Running Tests

To run all tests:
```bash
go test ./...
```

## API Endpoints

The application provides the following HTTP endpoints:

- `GET /` - Display all saved links
- `GET /?s=term` - Search for links
- `POST /` - Add a new link
- `GET /{id}` - Get a specific link
- `DELETE /{id}` - Delete a specific link

## Dependencies

- **Go 1.24**
- **modernc.org/sqlite**: Pure Go SQLite driver
- **Bootstrap**: CSS framework for responsive design
- **HTMX**: Dynamic HTML updates without JavaScript
- **Hyperscript**: Friendly client scripting
- **chromedp**: Run headless Chrome browser to fetch page, extract title, description and take screenshot

## License

Copyright 2025 Mikael Ståldal.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
