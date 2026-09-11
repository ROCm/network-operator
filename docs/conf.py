"""Configuration file for the Sphinx documentation builder."""

project = "Network Operator"
copyright = "Copyright (c) 2025 Advanced Micro Devices, Inc. All rights reserved."
author = "Shrey Ajmera, Akhila Yeruva"

import os
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
    "flavor": "instinct-design",
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

# Generate llms.txt and llms-full.txt after each build (the llms.txt standard,
# https://llmstxt.org/). See the rocm-docs-core guide:
# https://rocm.docs.amd.com/projects/rocm-docs-core/en/latest/user_guide/llms.html
rocm_docs_generate_llms = True

# Only for new projects. Remove when stable.
nitpicky = True

# Tags settings
tags_create_tags = True
tags_extension = ["md"]
tags_create_badges = True
tags_intro_text = ""
tags_page_title = "Tag page"
tags_page_header = "Pages with this tag"
