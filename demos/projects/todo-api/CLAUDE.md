# Task: Add a /search endpoint

Add a `GET /todos/search` endpoint that accepts a `q` query parameter and returns all TODO items whose title contains the search string (case-insensitive).

## Requirements

- The endpoint should be `GET /todos/search?q=<search_term>`
- Return a list of matching `TodoResponse` objects
- The search should be case-insensitive
- If `q` is empty or missing, return all items
- Add the endpoint to the existing `app.py` file
