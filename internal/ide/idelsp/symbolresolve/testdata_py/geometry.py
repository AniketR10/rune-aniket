import math


def area(radius):
    return math.pi * radius * radius


def perimeter(radius):
    # Defined but never referenced as geometry.perimeter; resolves
    # only through the definitions phase.
    return 2 * math.pi * radius


class Shape:
    def __init__(self, size):
        self.size = size

    def area(self):
        return self.size * self.size


class _Internal:
    # Leading-underscore name: Python has no enforced visibility, so
    # the resolver must still surface it.
    pass


def _make_internal():
    return _Internal()
