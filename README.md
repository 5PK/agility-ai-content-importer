# Agility AI Content Importer

Content item sidebar app for Agility CMS. The app is written in Go, renders HTML with templ, and uses htmx for multipart document uploads.

## Run locally

```bash
make dev
```

Open:

- `http://localhost:8080/content-item-sidebar`
- `http://localhost:8080/install`

The sidebar processes `.docx` files by extracting the raw OpenXML, sending it with the current Agility content item/model context to a local Ollama server, and applying the returned field values through the Agility app bridge. TXT files are accepted by the UI but are not processed yet.

Ollama defaults:

- `OLLAMA_URL=http://localhost:11434`
- `OLLAMA_MODEL=llama3.1`

The DOCX mapping prompts live in `prompts/docx-system.md` and `prompts/docx-user.md`.

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
