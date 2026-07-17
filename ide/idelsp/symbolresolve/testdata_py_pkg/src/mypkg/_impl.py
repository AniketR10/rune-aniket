class Widget:
    def __init__(self, name):
        self.name = name

    def render(self):
        return "<" + self.name + ">"


def make_widget(name):
    return Widget(name)