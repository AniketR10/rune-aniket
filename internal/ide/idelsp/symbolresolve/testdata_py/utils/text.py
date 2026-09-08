def slugify(value):
    return value.strip().lower().replace(" ", "-")


def _private_helper(value):
    # Leading underscore: still resolvable (no visibility rules).
    return value
