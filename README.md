# 👉 **See the live menu page here: [menu.krenn.dev](https://menu.krenn.dev)** 👈

# Go Menu Extractor

Easily view the weekly menus from two restaurants—JKU Mensa and KHG Mensa—in one convenient place. This application is designed to automatically collect the latest menu data from both locations and update the website [menu.krenn.dev](https://menu.krenn.dev) every day. The page is hosted with GitHub Pages and always shows the current week's menus side by side, so visitors never need to run the program or check multiple sites.

Menus are fetched live from:
- JKU Mensa (via GraphQL API)
- KHG Mensa (via HTML scraping)

The result is a combined HTML file (`menu_for_week_tabs.html`) with tabs for each weekday, showing what both restaurants offer side by side. This file is published daily to [menu.krenn.dev](https://menu.krenn.dev).

## Features
- Fetches JKU Mensa menu using a GraphQL POST request
- Scrapes KHG Mensa menu from a public HTML page
- Combines both menus into a single HTML file with tabs for each weekday
- Uses Go templates for HTML rendering

## Usage

### Build for Linux amd64 (required for GitHub Actions)
```sh
GOOS=linux GOARCH=amd64 go build -o build/creator && chmod +x build/creator
```

### Run
```sh
./go-menu-extractor
```
This will generate `menu_for_week_tabs.html` in the project directory.

## Project Structure
- `main.go` — Entry point, combines menus and writes HTML
- `fetch.go` — Fetches and parses menus from JKU and KHG
- `menu_for_week_tabs.tmpl` — Go template for rendering the HTML output
- `menu_for_week_tabs.html` — Generated output file
- `samplereqresp/` — Sample HTML files for reference

## Requirements
- Go 1.18+
- Internet connection (to fetch live menus)

## Dependencies
- [goquery](https://github.com/PuerkitoBio/goquery) — HTML parsing

Install dependencies:
```sh
go get github.com/PuerkitoBio/goquery
```

## License
MIT
