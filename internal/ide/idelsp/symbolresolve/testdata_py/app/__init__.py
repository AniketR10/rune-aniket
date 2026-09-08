class App:
    # Defined in a package __init__.py; the module qualifier is the
    # parent directory name ("app"), so this resolves as app.App.
    def __init__(self):
        self.name = "app"


def create():
    return App()
