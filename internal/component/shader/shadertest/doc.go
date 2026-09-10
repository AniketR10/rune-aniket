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

/*
Package shadertest provides testing harnesses to produce suites to run on
the shaders you create as well as to provide utilities to create the data
for those tests.

When creating your shader make sure to test it by calling the suite harness:

	func TestCoolEffects(t *testing.T) {
		// run the general intesive suite
		shadertest.TestShader(t, shader.CoolEffects())

		// other tests particular to CoolEffects shader go below:
		sh := CoolEffects(DefaultCoolEffectsParams())

		// [...]
	}

If importing shadertest package into your shader test introduces cyclic
dependency then place the test file within shadertest package as
shader_cool_effects_test.go (in this case):

	func TestCoolEffects(t *testing.T) {
		TestShader(t, shader.CoolEffects())
	}
*/
package shadertest
