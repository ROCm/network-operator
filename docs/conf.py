"""Configuration file for the Sphinx documentation builder."""

project = "Network Operator"
copyright = "Copyright (c) 2025 Advanced Micro Devices, Inc. All rights reserved."
author = "Shrey Ajmera, Akhila Yeruva"

import os
from pathlib import Path
import shutil
html_baseurl = os.environ.get("READTHEDOCS_CANONICAL_URL", "instinct.docs.amd.com")
html_context = {}
if os.environ.get("READTHEDOCS", "") == "True":
    html_context["READTHEDOCS"] = True

version = "0.1.0"
release = version
html_title = project
external_projects_current_project = "network-operator"

# Required settings
html_theme = "rocm_docs_theme"
html_theme_options = {
    "flavor": "instinct",
    "link_main_doc": True,
    "use_download_button": True,
    # Add any additional theme options here
}
extensions = [
    "rocm_docs",
    "sphinx_tags",
]

# Table of contents
external_toc_path = "./sphinx/_toc.yml"
external_toc_exclude_missing = False

# Only for new projects. Remove when stable.
nitpicky = True

# Tags settings
tags_create_tags = True
tags_extension = ["md"]
tags_create_badges = True
tags_intro_text = ""
tags_page_title = "Tag page"
tags_page_header = "Pages with this tag"

EXCLUDED_DIRS = {
    "_build",
    "_templates",
    "_static",
    ".git",
    ".venv",
}

def should_skip(path: Path) -> bool:
    return any(part in EXCLUDED_DIRS for part in path.parts)


def generate_combined_markdown(app, exception):
    if exception:
        return

    docs_root = Path(app.srcdir)
    output_file = Path(app.outdir) / "llms.txt"

    print(output_file)

    all_files = sorted(docs_root.rglob("*.md"))

    combined = []
    combined.append("# Combined Documentation\n")

    for doc_file in all_files:
        if should_skip(doc_file):
            continue

        relative = doc_file.relative_to(docs_root)

        combined.append(f"\n---\n")
        combined.append(f"\n# {relative}\n")

        try:
            content = doc_file.read_text(encoding="utf-8")
            combined.append(content)
            combined.append("\n")

        except Exception as e:
            combined.append(f"\n[ERROR reading file: {e}]\n")

    output_file.write_text(
        "\n".join(combined),
        encoding="utf-8",
    )

def setup(app):
    app.connect("build-finished", generate_combined_markdown)
