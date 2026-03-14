#!/usr/bin/env python3
"""Sample Python file for syntax highlighting."""

def fibonacci(n: int) -> int:
    """Return the nth Fibonacci number."""
    if n <= 1:
        return n
    a, b = 0, 1
    for _ in range(2, n + 1):
        a, b = b, a + b
    return b

class Calculator:
    def __init__(self):
        self.history = []

    def add(self, x, y):
        result = x + y
        self.history.append(result)
        return result

if __name__ == "__main__":
    calc = Calculator()
    print(calc.add(1, 2))
    print(fibonacci(10))
