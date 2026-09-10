// Copyright (C) 2017-2026 The Rune Authors
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package main

import "fmt"

// Greeter greets people.
type Greeter struct {
	Name string
}

// Greet returns a greeting message.
func (g *Greeter) Greet() string {
	return fmt.Sprintf("Hello, %s!", g.Name)
}

// Add adds two integers.
func Add(a, b int) int {
	return a + b
}

func main() {
	g := &Greeter{Name: "World"}
	fmt.Println(g.Greet())
	fmt.Println(Add(1, 2))
}

// Speaker is an interface for things that can speak.
// See also [Add] for arithmetic operations.
type Speaker interface {
	Speak() string
}

// Robot is a mechanical speaker.
type Robot struct {
	ID int
}

// Speak returns a robotic greeting.
func (r *Robot) Speak() string {
	return fmt.Sprintf("Beep boop, I am robot %d", r.ID)
}
