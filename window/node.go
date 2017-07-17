package window

import (
	"fmt"
	"math"
)

type winode struct {
	window *TiledWindow
	lsplit *winode
	rsplit *winode
}

func newWinode(t *TiledWindow) *winode {
	node := &winode{t, nil, nil}
	t.node = node
	return node
}

func (n *winode) Draw() error {
	if n.window != nil {
		return n.window.Draw()
	}

	if n.lsplit == nil || n.rsplit == nil {
		panic(fmt.Sprintf("invalid node found while drawing: %+v", n))
	}

	if err := n.lsplit.Draw(); err != nil {
		return err
	}

	return n.rsplit.Draw()
}

// FIXME instead of using rounding error it should use tree relationships
// to figure out which is its node on the left and on the top,
// so it knows exactly where it starts and where it ends
func (n *winode) ResizeMove(xfactor, yfactor, xerror, yerror float64) (float64, float64, error) {
	var err error
	if n.window != nil {
		newWidth := float64(n.window.Width())*xfactor + xerror
		newHeight := float64(n.window.Height())*yfactor + yerror
		truncWidth := int(math.Trunc(newWidth))
		truncHeight := int(math.Trunc(newHeight))

		if err = n.window.Resize(truncWidth, truncHeight); err != nil {
			return xerror, yerror, err
		}

		xpos, ypos := n.window.Position()
		newPosx := float64(xpos)*xfactor - xerror
		newPosy := float64(ypos)*yfactor - yerror
		truncPosx := int(math.Trunc(newPosx))
		truncPosy := int(math.Trunc(newPosy))
		n.window.Move(truncPosx, truncPosy)

		// calc size rounding errors
		xerror = newWidth - float64(truncWidth) + newPosx - float64(truncPosx)
		yerror = newHeight - float64(truncHeight) + newPosy - float64(truncPosy)

		return xerror, yerror, nil
	}

	if n.lsplit == nil || n.rsplit == nil {
		panic(fmt.Sprintf("invalid node found while resizing: %+v", n))
	}

	if xerror, yerror, err =
		n.lsplit.ResizeMove(xfactor, yfactor, xerror, yerror); err != nil {
		return xerror, yerror, err
	}

	if xerror, yerror, err =
		n.rsplit.ResizeMove(xfactor, yfactor, xerror, yerror); err != nil {
		return xerror, yerror, err
	}

	return xerror, yerror, nil
}
