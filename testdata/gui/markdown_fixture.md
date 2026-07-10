# Zephyr GUI Regression

This fixture validates **Markdown read mode**, pointer selection, and transitions
between the rendered document and its source editor.

## Input behavior

- Primary drags select rendered text.
- The hidden editor cursor must not move in read mode.
- `Cmd+E` toggles Edit and Read without losing a frame.

| Scenario | Expected result |
|---|---|
| Read mode | Complete rendered document |
| Edit mode | Complete source editor |
| Return to Read | Complete rendered document |

```go
func regressionFixture() string {
	return "tabs and syntax highlighting"
}
```

Enough trailing content is included to exercise scrolling and redraw behavior.
