# orcha

Unix pipes for AI workflows. Define reusable tasks and linear pipelines in `orcha.yaml`; run them from Python with a single line.

```python
from orcha import Orcha

o = Orcha("./orcha.yaml")
for event in o.run("summarize-and-translate", "./article.txt"):
    print(event)
```

The package ships a tiny Python wrapper; the actual engine is a Go binary that's auto-downloaded to `~/.orcha/bin/` on first use.
