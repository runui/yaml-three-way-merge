package corpus

func FifteenStates() []State {
	absent := Atom{}
	a := Atom{Present: true, Value: 0}
	u := Atom{Present: true, Value: 1}
	r := Atom{Present: true, Value: 2}
	return []State{
		{ID: "01-all-absent", Base: absent, User: absent, Remote: absent, Expected: absent},
		{ID: "02-remote-add", Base: absent, User: absent, Remote: r, Expected: r},
		{ID: "03-user-add", Base: absent, User: u, Remote: absent, Expected: u},
		{ID: "04-both-add-same", Base: absent, User: u, Remote: u, Expected: u},
		{ID: "05-both-add-different-user-wins", Base: absent, User: u, Remote: r, Expected: u},
		{ID: "06-all-unchanged", Base: a, User: a, Remote: a, Expected: a},
		{ID: "07-remote-modify", Base: a, User: a, Remote: r, Expected: r},
		{ID: "08-remote-delete", Base: a, User: a, Remote: absent, Expected: absent},
		{ID: "09-user-modify", Base: a, User: u, Remote: a, Expected: u},
		{ID: "10-user-delete", Base: a, User: absent, Remote: a, Expected: absent},
		{ID: "11-both-modify-same", Base: a, User: u, Remote: u, Expected: u},
		{ID: "12-both-modify-different-user-wins", Base: a, User: u, Remote: r, Expected: u},
		{ID: "13-user-modify-remote-delete", Base: a, User: u, Remote: absent, Expected: u},
		{ID: "14-user-delete-remote-modify", Base: a, User: absent, Remote: r, Expected: absent},
		{ID: "15-both-delete", Base: a, User: absent, Remote: absent, Expected: absent},
	}
}

func Resolve(base, user, remote Atom) Atom {
	if base == user {
		return remote
	}
	return user
}
