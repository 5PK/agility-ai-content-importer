# Agility AI Content Importer

Content item sidebar app for Agility CMS. The app is written in Go, renders HTML with templ, and uses htmx for multipart document uploads.

## Run locally

```bash
make dev
```

Open:

- `http://localhost:8080/content-item-sidebar`
- `http://localhost:8080/install`

The sidebar currently accepts `.docx` and `.txt` files and returns a placeholder upload result. Document parsing and field updates can be added behind `POST /content-item-sidebar/upload`.

## Make targets

```bash
make generate
make check
make build
make clean
```

Use `PORT=3000 make dev` to run on a different port.
# agility-ai-content-importer
# agility-ai-content-importer
