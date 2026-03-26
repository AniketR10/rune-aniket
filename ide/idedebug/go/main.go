// Copyright 2026 Unstable Build, LLC.
//
// This program is free software: you can redistribute it and/or modify it
// under the terms of the GNU General Public License as published by the
// Free Software Foundation, either version 3 of the License, or (at your
// option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// See <https://www.gnu.org/licenses/> for a copy of the license.

package main

import "fmt"

// Add adds two integers.
func Add(a, b int) int {
	return a + b
}

func main() {
	x := 10
	y := 20
	sum := Add(x, y)
	fmt.Println("Sum:", sum)
}
