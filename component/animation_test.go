package component

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"unstable.build/go-tui/term"
)

func newTestInterrupter() (term.Interrupter, chan struct{}) {
	ch := make(chan struct{})
	interrupter := term.FuncInterrupter(func() error {
		ch <- struct{}{}
		return nil
	})
	return interrupter, ch
}

func TestAnimation(t *testing.T) {
	suite := []struct {
		desc         string
		newAnimation func(*testing.T, term.Interrupter, []string, []int, int) *Animation
	}{
		{"NewAnimation constructor",
			func(t *testing.T, interrupter term.Interrupter, frames []string, sequence []int, fps int) *Animation {
				return NewAnimation(interrupter, frames, sequence, fps)
			}},
		{"encoded and decoded animation",
			func(t *testing.T, interrupter term.Interrupter, frames []string, sequence []int, fps int) *Animation {
				a := NewAnimation(interrupter, frames, sequence, fps)
				defer a.Close()

				raw := EncodeAnimation(a)
				ret, err := DecodeAnimation(raw, fps, interrupter)
				require.NoError(t, err)
				return ret
			}},
	}
	for _, test := range suite {
		test := test
		t.Run(test.desc, func(t *testing.T) {

			t.Run("no frames", func(t *testing.T) {
				fps := 30
				interrupter, _ := newTestInterrupter()

				frames := []string{}
				sequence := []int{}
				expected := "        \n        \n        \n        "

				// sut
				c := NewAnimation(interrupter, frames, sequence, fps)
				c.Resize(8, 4)

				for i := 0; i < 30; i++ {
					w := term.NewStringWriter(8, 4)
					c.Draw(w)

					require.NoError(t, w.Flush())
					assert.Equal(t, expected, w.String())
				}

				require.NoError(t, c.Close())
				goleak.VerifyNone(t)
			})

			t.Run("single static frame", func(t *testing.T) {
				fps := 30
				interrupter, ch := newTestInterrupter()

				frames := []string{"1111    \n1111    \n    @@@@\n    @@@@"}
				sequence := []int{0}
				expected := "1111    \n1111    \n    @@@@\n    @@@@"

				// sut
				c := NewAnimation(interrupter, frames, sequence, fps)
				c.Resize(8, 4)

				for i := 0; i < 30; i++ {
					w := term.NewStringWriter(8, 4)
					<-ch
					c.Draw(w)

					require.NoError(t, w.Flush())
					assert.Equal(t, expected, w.String())
				}

				require.NoError(t, c.Close())
				goleak.VerifyNone(t)
			})

			t.Run("multiple frames, repeated or not", func(t *testing.T) {
				fps := 30
				interrupter, ch := newTestInterrupter()

				frames := []string{
					"0000    \n0000    \n    0000\n    0000",
					"1111    \n1111    \n    1111\n    1111",
				}
				sequence := []int{0, 0, 1}

				// sut
				c := test.newAnimation(t, interrupter, frames, sequence, fps)
				c.Resize(8, 4)

				for i := 0; i < 30; i++ {
					w := term.NewStringWriter(8, 4)
					<-ch
					c.Draw(w)
					var expected string
					switch i % 3 {
					case 0:
						expected = "0000    \n0000    \n    0000\n    0000"
					case 1:
						expected = "0000    \n0000    \n    0000\n    0000"
					case 2:
						expected = "1111    \n1111    \n    1111\n    1111"
					default:
						panic("hmmm")
					}

					require.NoError(t, w.Flush())
					assert.Equal(t, expected, w.String())
				}

				require.NoError(t, c.Close())
				goleak.VerifyNone(t)
			})
		})
	}
}
