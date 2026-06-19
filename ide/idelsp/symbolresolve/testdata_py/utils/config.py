# "config" is an ambiguous module name shared with app/config.py.
# A definition-phase lookup of config.load must surface both files.
SETTINGS = {}


def load():
    return dict(SETTINGS)


def reset():
    SETTINGS.clear()
