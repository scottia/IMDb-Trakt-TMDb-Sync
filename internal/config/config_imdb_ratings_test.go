package config

import "testing"

func TestIMDbRatingsUserListDefaultsEmpty(t *testing.T) {
	conf := destinationTestConfig(t, map[string]interface{}{})
	if conf.IMDb.RatingsUserList == nil {
		t.Fatal("IMDB_RATINGSUSERLIST default is nil")
	}
	if *conf.IMDb.RatingsUserList != "" {
		t.Fatalf("IMDB_RATINGSUSERLIST default = %q, want empty", *conf.IMDb.RatingsUserList)
	}
}

func TestIMDbRatingsUserListLoadsFromMap(t *testing.T) {
	conf := destinationTestConfig(t, map[string]interface{}{
		"IMDB_RATINGSUSERLIST": "https://www.imdb.com/user/ur1234567/ratings",
	})
	if conf.IMDb.RatingsUserList == nil || *conf.IMDb.RatingsUserList == "" {
		t.Fatal("IMDB_RATINGSUSERLIST was not loaded")
	}
}
