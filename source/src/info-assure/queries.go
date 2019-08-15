package main

//
func getUsers() ([]*User, error) {

	var data []*User
	var err error

	err = db.
		Select("*").
		From("users").
		OrderBy("username ASC").
		QueryStructs(&data)

	for _, u := range data {
		u.Beautify()
	}

	return data, err
}

//
func getHosts(host string) ([]*Instance, error) {

	var data []*Instance
	var err error

	// for i := 1; i <= 100000; i++ {
	// 	data = append(data, &Instance{Host: fmt.Sprintf("%d", i)})
	// }

	// return data, err

	err = db.
		Select("host").
		Distinct().
		From("instance").
		Where("host LIKE $1", "%"+host+"%").
		OrderBy("host ASC").
		QueryStructs(&data)

	return data, err
}
