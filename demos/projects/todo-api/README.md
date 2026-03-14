# TODO API

A lightweight task management API built with FastAPI.

## Quick Start

```bash
pip install -r requirements.txt
uvicorn app:app --reload
```

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/todos` | List all items |
| POST | `/todos` | Create a new item |
| GET | `/todos/{id}` | Get a single item |
| PATCH | `/todos/{id}` | Update an item |
| DELETE | `/todos/{id}` | Delete an item |

## Example

```bash
# Create a todo
curl -X POST http://localhost:8000/todos \
  -H "Content-Type: application/json" \
  -d '{"title": "Buy groceries"}'

# List all todos
curl http://localhost:8000/todos
```
