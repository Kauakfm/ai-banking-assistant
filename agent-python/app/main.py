"""
AI Banking Assistant — Ponto de entrada (Padrão BFA).

Fluxo end-to-end:
1. FastAPI startup: indexa RAG + conecta ao BFA via MCP + compila supervisor
2. POST /generate: recebe {customer_id, query}
3. Supervisor (LangGraph): guardrails → planner → sub-agentes → formatter
4. Sub-agentes chamam BFA via MCP para dados de domínio

  Cliente → POST /generate → Supervisor → Sub-agentes → BFA (MCP) → BFA Go
"""

import os
from contextlib import asynccontextmanager

from fastapi import FastAPI, HTTPException, Request
from pydantic import BaseModel
from langchain_core.messages import HumanMessage
from langchain_mcp_adapters.client import MultiServerMCPClient

# Compatibility shim: langfuse v2 imports from langchain.callbacks.base /
# langchain.schema.agent / langchain.schema.document, which were removed
# in LangChain v1.x. The shim re-maps them to langchain_core equivalents.
from app.langchain_compat import install_langchain_compat_shim
install_langchain_compat_shim()

from langfuse.callback import CallbackHandler  # noqa: E402  — after shim

from app.agent.tools import rag_search
from app.agent.supervisor import build_supervisor
from app.rag.indexer import get_vector_store


@asynccontextmanager
async def lifespan(app: FastAPI):
    # --- RAG: indexar base de conhecimento ---
    print("[Startup] Inicializando e indexando base de conhecimento (RAG)...")
    get_vector_store()
    print("[Startup] RAG carregado com sucesso.")

    # --- MCP: conectar ao BFA Server ---
    print("[Startup] Conectando ao BFA via MCP (protocolo Model Context Protocol)...")
    mcp_client = MultiServerMCPClient(
        {
            "bfa": {
                "command": "python",
                "args": ["-m", "app.mcp.bfa_server"],
                "transport": "stdio",
            }
        }
    )
    mcp_tools = await mcp_client.get_tools()
    print(f"[Startup] MCP conectado. {len(mcp_tools)} ferramenta(s) carregada(s) do BFA.")
    for tool in mcp_tools:
        print(f"  -> MCP Tool: {tool.name}")

    # --- Supervisor: compilar grafo LangGraph com ferramentas MCP + RAG ---
    print("[Startup] Compilando grafo do Supervisor...")
    supervisor = build_supervisor(mcp_tools, rag_search)
    app.state.supervisor = supervisor
    app.state.mcp_client = mcp_client
    print("[Startup] Supervisor pronto. Servidor disponível.")

    yield

    print("[Shutdown] Encerrando agente e conexões MCP...")


app = FastAPI(title="AI Banking Assistant — Supervisor Agent (BFA)", lifespan=lifespan)


class ContextRequest(BaseModel):
    customer_id: str
    query: str


@app.post("/generate")
async def generate_response(body: ContextRequest, request: Request):
    """
    Endpoint principal do assistente bancário.

    Recebe: {customer_id: str, query: str}
    Retorna: resposta contextualizada com justificativa estruturada.

    O supervisor (LangGraph) decide quais sub-agentes acionar
    e chama o BFA via MCP para obter dados de domínio.
    """
    try:
        initial_state = {
            "customer_id": body.customer_id,
            "query": body.query,
            "messages": [HumanMessage(content=body.query)],
            "tools_used": [],
            "tool_call_count": 0,
            "risk_score": 0.0,
            "agents_to_call": [],
            "profile_result": "",
            "transaction_result": "",
            "knowledge_result": "",
        }

        config = {}
        langfuse_handler = None
        if os.getenv("LANGFUSE_PUBLIC_KEY") and os.getenv("LANGFUSE_SECRET_KEY"):
            langfuse_handler = CallbackHandler(
                trace_name="banking-assistant-supervisor",
                session_id=f"session-{body.customer_id}",
                user_id=body.customer_id,
                tags=["banking-assistant", "supervisor"],
                metadata={
                    "customer_id": body.customer_id,
                    "query": body.query,
                },
            )
            config = {"callbacks": [langfuse_handler]}

        supervisor = request.app.state.supervisor
        result_state = await supervisor.ainvoke(initial_state, config=config)

        # Flush para garantir que todos os eventos sejam enviados ao LangFuse
        if langfuse_handler:
            langfuse_handler.flush()

        final_answer = result_state.get("final_answer", "")
        if not final_answer:
            final_answer = (
                f"Justificativa: {result_state.get('justification', '')}. "
                f"Ferramentas utilizadas: {', '.join(result_state.get('tools_used', [])) or 'nenhuma'}"
            )

        return {
            "customer_id": body.customer_id,
            "query": body.query,
            "response": final_answer,
            "metadata": {
                "justification": result_state.get("justification", ""),
                "tools_used": result_state.get("tools_used", []),
                "plan": result_state.get("plan", ""),
                "risk_score": result_state.get("risk_score", 0.0),
                "agents_called": result_state.get("agents_to_call", []),
            },
        }
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))