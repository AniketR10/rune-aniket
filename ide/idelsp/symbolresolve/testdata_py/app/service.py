from utils import text


class Service:
    def __init__(self):
        self.label = text.slugify("service")

    def run(self):
        # self.helper is an attribute access whose object is "self";
        # it must not be mistaken for a module-qualified reference to
        # a "self" module.
        return self.helper()

    def helper(self):
        return self.label


def boot():
    return Service()
