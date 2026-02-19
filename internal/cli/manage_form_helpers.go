package cli

func resizeFormInput(f *manageForm, width int) *manageForm {
	if f == nil {
		return nil
	}
	f.Input.Width = clampInt(width-8, 20, 120)
	return f
}
