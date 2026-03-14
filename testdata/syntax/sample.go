package main

import "fmt"

// fibonacci returns the nth Fibonacci number.
func fibonacci(n int) int {
	if n <= 1 {
		return n
	}
	a, b := 0, 1
	for i := 2; i <= n; i++ {
		a, b = b, a+b
	}
	return b
}

func main() {
	for i := 0; i < 10; i++ {
		fmt.Printf("F(%d) = %d\n", i, fibonacci(i))
	}
}
