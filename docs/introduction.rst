============
Introduction
============

Vile is a multi-paradigm general-purpose programming language in the `Lisp family <https://en.wikipedia.org/wiki/Lisp_(programming_language)>`_.
Vile is an imperative, functional and reflective programming language which allow you to write simple and readable code however Vile is lightweight and simple.

Some numerical program implemented in Vile:

.. fibonacci-recursion:

::

	(fn fibonacci_recursion(n)
		(if (<= n 1) n
			(+ (fibonacci_recursion (- n 1)) (fibonacci_recursion (- n 2))))
	)

	(puts (fibonacci_recursion 8))

.. factorial:

::

	(fn factorial(n)
		(if (= n 1) n
			(if (= n 0) n
				(* n (factorial (- n 1)))))
	)

	(puts (factorial 5))
