import geometry


class User:
    def __init__(self, name):
        self.name = name


class Account:
    def __init__(self, owner):
        self.owner = owner
        # Cross-module qualified reference: geometry.area is referenced
        # here as well as in main.py.
        self.bounds = geometry.area(1.0)
