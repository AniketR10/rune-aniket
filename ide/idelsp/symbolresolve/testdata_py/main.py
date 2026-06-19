import geometry
import requests

from app import service
from utils import text


def main():
    # Qualified references: resolve to this call site via the
    # reference (attribute) phase.
    area = geometry.area(3.0)
    shape = geometry.Shape(area)

    # Dependency module reference (requests is a sibling "package").
    resp = requests.get("https://example.com")

    svc = service.Service()
    slug = text.slugify("Hello World")
    return shape, resp, svc, slug


if __name__ == "__main__":
    main()
