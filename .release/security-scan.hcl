container {
	secrets {
		all = false
	}

       dependencies    = false
       alpine_security = false
}

binary {
	go_modules = true
	osv        = true
	nvd        = true

	secrets {
		all = true
	}
}