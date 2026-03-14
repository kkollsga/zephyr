// Sample Rust file for syntax highlighting

fn fibonacci(n: u64) -> u64 {
    if n <= 1 {
        return n;
    }
    let (mut a, mut b) = (0u64, 1u64);
    for _ in 2..=n {
        let temp = b;
        b = a + b;
        a = temp;
    }
    b
}

struct Calculator {
    history: Vec<i64>,
}

impl Calculator {
    fn new() -> Self {
        Calculator { history: Vec::new() }
    }

    fn add(&mut self, x: i64, y: i64) -> i64 {
        let result = x + y;
        self.history.push(result);
        result
    }
}

fn main() {
    let mut calc = Calculator::new();
    println!("{}", calc.add(1, 2));
    println!("{}", fibonacci(10));
}
