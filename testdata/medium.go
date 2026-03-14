package main

import (
	"fmt"
	"math"
	"strings"
)

// fibonacci returns the nth Fibonacci number using dynamic programming.
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

// isPrime checks if a number is prime.
func isPrime(n int) bool {
	if n < 2 {
		return false
	}
	for i := 2; i <= int(math.Sqrt(float64(n))); i++ {
		if n%i == 0 {
			return false
		}
	}
	return true
}

// reverseString reverses a string using rune slice.
func reverseString(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// Matrix represents a 2D matrix.
type Matrix struct {
	rows, cols int
	data       [][]float64
}

// NewMatrix creates a new matrix filled with zeros.
func NewMatrix(rows, cols int) *Matrix {
	data := make([][]float64, rows)
	for i := range data {
		data[i] = make([]float64, cols)
	}
	return &Matrix{rows: rows, cols: cols, data: data}
}

// Multiply multiplies two matrices.
func (m *Matrix) Multiply(other *Matrix) (*Matrix, error) {
	if m.cols != other.rows {
		return nil, fmt.Errorf("incompatible dimensions: %dx%d * %dx%d",
			m.rows, m.cols, other.rows, other.cols)
	}
	result := NewMatrix(m.rows, other.cols)
	for i := 0; i < m.rows; i++ {
		for j := 0; j < other.cols; j++ {
			sum := 0.0
			for k := 0; k < m.cols; k++ {
				sum += m.data[i][k] * other.data[k][j]
			}
			result.data[i][j] = sum
		}
	}
	return result, nil
}

func main() {
	// Print first 20 Fibonacci numbers
	fmt.Println("Fibonacci sequence:")
	for i := 0; i < 20; i++ {
		fmt.Printf("  F(%d) = %d\n", i, fibonacci(i))
	}

	// Find primes up to 50
	fmt.Println("\nPrimes up to 50:")
	var primes []string
	for i := 2; i <= 50; i++ {
		if isPrime(i) {
			primes = append(primes, fmt.Sprintf("%d", i))
		}
	}
	fmt.Println("  " + strings.Join(primes, ", "))

	// Reverse a string
	original := "Hello, 世界! 🌍"
	fmt.Printf("\nOriginal: %s\n", original)
	fmt.Printf("Reversed: %s\n", reverseString(original))
}
