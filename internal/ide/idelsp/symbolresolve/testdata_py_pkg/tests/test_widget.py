from mypkg import Widget, make_widget


def test_widget_render():
    assert Widget("a").render() == "<a>"


def test_make_widget():
    widget = make_widget("b")
    assert widget.name == "b"