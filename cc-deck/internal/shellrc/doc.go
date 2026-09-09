// Package shellrc writes and manages idempotent, marker-delimited blocks in
// shell rc files (~/.bashrc, ~/.zshrc). The markers are:
//
//	# >>> cc-deck >>>
//	<content>
//	# <<< cc-deck <<<
//
// Ensure writes or replaces the block while preserving all surrounding content
// byte for byte. EnsureAll targets both .bashrc and .zshrc under a given home
// directory.
package shellrc
