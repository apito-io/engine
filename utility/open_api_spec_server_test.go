package utility

import "testing"

func TestRestDocServerURL(t *testing.T) {
	if got := restDocServerURL("local", "", "p1"); got != "http://localhost:5050/secured/rest/p1" {
		t.Fatal(got)
	}
	if got := restDocServerURL("production", "", "p1"); got != "https://api.apito.io/secured/rest/p1" {
		t.Fatal(got)
	}
	if got := restDocServerURL("production", "https://engine.udbhabon.com/", "p1"); got != "https://engine.udbhabon.com/secured/rest/p1" {
		t.Fatal(got)
	}
}
