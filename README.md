# Vile

<a href="https://github.com/sami2020pro/vile/blob/main/data/vile.png">
    <img
        src="data/vile.png"
        raw=true
        alt="Vile lisp dialect"
        style="margin-right: 10px;"
    />
</a>

# Hello World
`Hello, World!` example in **Vile**:

```
(puts "Hello, World!")
```

# Preview
`Fibonacci Recursion` in **Vile**:

```
(fn fibonacci_recursion(n)
        (if (<= n 1) n
                (+ (fibonacci_recursion (- n 1)) (fibonacci_recursion (- n 2))))
)

(puts (fibonacci_recursion 8))
```

`Factorial` in **Vile**:

```
(fn factorial(n)
        (if (= n 1) n
                (if (= n 0) n
                        (* n (factorial (- n 1)))))
)

(puts (factorial 0))
(puts (factorial 1))
(puts (factorial 3))
(puts (factorial 4))
(puts (factorial 5))
```

# Getting started
If you have **Go** installed on your device, you can install **Vile** easily:

```
go get -u github.com/sami2020pro/vile/...
```
