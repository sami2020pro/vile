package vile

func RunStringEval(vileCode string) error {
	vileCodeC := String(string(vileCode))

	x, err := ReadAll(vileCodeC, nil)
	if err != nil {
		return err
	}

	for x != EmptyList {
                expr := Car(x)
                _, err := Eval(expr)
                if err != nil {
                        return err
                }

                x = Cdr(x)
        }
	return nil
}
