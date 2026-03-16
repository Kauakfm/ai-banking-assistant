"""
Compatibility shim: langchain v1.x removed langchain.callbacks.base,
langchain.schema.agent and langchain.schema.document.
Langfuse SDK v2 still imports from those paths.
This module re-maps them to langchain_core equivalents.

Must be imported BEFORE any langfuse callback import.
"""
import sys
import types


def install_langchain_compat_shim():
    """Install compatibility shim for langfuse v2 + langchain v1.x."""

    if "langchain.callbacks.base" not in sys.modules:
        try:
            import langchain.callbacks.base  # noqa: F401
        except (ImportError, ModuleNotFoundError):
            import langchain
            import langchain_core.callbacks.base

            if not hasattr(langchain, "callbacks"):
                callbacks_mod = types.ModuleType("langchain.callbacks")
                callbacks_mod.__path__ = []
                langchain.callbacks = callbacks_mod
                sys.modules["langchain.callbacks"] = callbacks_mod

            sys.modules["langchain.callbacks.base"] = langchain_core.callbacks.base

    if "langchain.schema.agent" not in sys.modules:
        try:
            import langchain.schema.agent  # noqa: F401
        except (ImportError, ModuleNotFoundError):
            import langchain
            import langchain_core.agents

            if not hasattr(langchain, "schema"):
                schema_mod = types.ModuleType("langchain.schema")
                schema_mod.__path__ = []
                langchain.schema = schema_mod
                sys.modules["langchain.schema"] = schema_mod

            sys.modules["langchain.schema.agent"] = langchain_core.agents

    if "langchain.schema.document" not in sys.modules:
        try:
            import langchain.schema.document  # noqa: F401
        except (ImportError, ModuleNotFoundError):
            import langchain
            import langchain_core.documents

            if not hasattr(langchain, "schema"):
                schema_mod = types.ModuleType("langchain.schema")
                schema_mod.__path__ = []
                langchain.schema = schema_mod
                sys.modules["langchain.schema"] = schema_mod

            sys.modules["langchain.schema.document"] = langchain_core.documents
