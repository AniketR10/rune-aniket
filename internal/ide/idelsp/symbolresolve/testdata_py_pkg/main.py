"""Plain script outside the package: no attribute references, so the
re-export bindings in the package __init__ stay the only match sites."""

from mypkg import make_widget


def main():
    widget = make_widget("cli")
    return widget.render()


if __name__ == "__main__":
    main()