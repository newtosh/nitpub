package main

import "testing"

// TestEnableQuotePostsFlagDefaultsFalse and TestEnableQuotePostsFlagSet cover
// U2's happy-path scenarios: the root command exposes --enable-quote-posts
// as a persistent bool flag defaulting to false, and setting it flips the
// parsed value to true. RunE threads this value straight into run() (see
// main.go), which is the only place Config.QuotePostsEnabled is ever set.
func TestEnableQuotePostsFlagDefaultsFalse(t *testing.T) {
	root := newRootCmd()
	got, err := root.PersistentFlags().GetBool("enable-quote-posts")
	if err != nil {
		t.Fatal(err)
	}
	if got {
		t.Fatal("--enable-quote-posts should default to false")
	}
}

func TestEnableQuotePostsFlagSet(t *testing.T) {
	root := newRootCmd()
	if err := root.PersistentFlags().Set("enable-quote-posts", "true"); err != nil {
		t.Fatal(err)
	}
	got, err := root.PersistentFlags().GetBool("enable-quote-posts")
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Fatal("--enable-quote-posts should be true after being set")
	}
}
