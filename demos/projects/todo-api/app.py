"""TODO API - A simple task management API built with FastAPI."""

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

app = FastAPI(title="TODO API", version="0.1.0")


class TodoItem(BaseModel):
    title: str
    done: bool = False


class TodoResponse(BaseModel):
    id: int
    title: str
    done: bool


todos: dict[int, TodoItem] = {}
next_id: int = 1


@app.get("/todos", response_model=list[TodoResponse])
def list_todos():
    """List all TODO items."""
    return [TodoResponse(id=tid, **t.model_dump()) for tid, t in todos.items()]


@app.post("/todos", response_model=TodoResponse, status_code=201)
def create_todo(item: TodoItem):
    """Create a new TODO item."""
    global next_id
    todos[next_id] = item
    resp = TodoResponse(id=next_id, **item.model_dump())
    next_id += 1
    return resp


@app.get("/todos/{todo_id}", response_model=TodoResponse)
def get_todo(todo_id: int):
    """Get a single TODO item by ID."""
    if todo_id not in todos:
        raise HTTPException(status_code=404, detail="Todo not found")
    return TodoResponse(id=todo_id, **todos[todo_id].model_dump())


@app.patch("/todos/{todo_id}", response_model=TodoResponse)
def update_todo(todo_id: int, item: TodoItem):
    """Update an existing TODO item."""
    if todo_id not in todos:
        raise HTTPException(status_code=404, detail="Todo not found")
    todos[todo_id] = item
    return TodoResponse(id=todo_id, **item.model_dump())


@app.delete("/todos/{todo_id}", status_code=204)
def delete_todo(todo_id: int):
    """Delete a TODO item."""
    if todo_id not in todos:
        raise HTTPException(status_code=404, detail="Todo not found")
    del todos[todo_id]
