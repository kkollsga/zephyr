// Sample JavaScript file for syntax highlighting

const fibonacci = (n) => {
  if (n <= 1) return n;
  let a = 0, b = 1;
  for (let i = 2; i <= n; i++) {
    [a, b] = [b, a + b];
  }
  return b;
};

class Calculator {
  constructor() {
    this.history = [];
  }

  add(x, y) {
    const result = x + y;
    this.history.push(result);
    return result;
  }
}

const calc = new Calculator();
console.log(calc.add(1, 2));
console.log(fibonacci(10));
