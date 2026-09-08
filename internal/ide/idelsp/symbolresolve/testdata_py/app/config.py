# Homonymous module with utils/config.py. config.load is defined in
# both; the resolver must surface both definition files for an
# ambiguous lookup, and must NOT cross-contaminate distinct names.
DEFAULTS = {"debug": False}


def load():
    return dict(DEFAULTS)


def save(data):
    DEFAULTS.update(data)
