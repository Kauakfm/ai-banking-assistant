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
import time
from contextlib import asynccontextmanager

from fastapi import FastAPI, HTTPException, Request, Response
from pydantic import BaseModel
from langchain_core.messages import HumanMessage
from langchain_mcp_adapters.client import MultiServerMCPClient

from app.langchain_compat import install_langchain_compat_shim
install_langchain_compat_shim()

from langfuse.callback import CallbackHandler  # noqa: E402

from app.agent.tools import rag_search
from app.agent.supervisor import build_supervisor
from app.rag.indexer import get_vector_store
from app.metrics import (
    REQUEST_LATENCY,
    REQUESTS_TOTAL,
    GUARDRAIL_BLOCKS,
    record_token_usage,
    metrics_response,
)
from app.security.middleware import (
    validate_input_length,
    rate_limiter,
    mask_pii,
    InputValidationError,
    RateLimitError,
)


OPENAI_MODEL = os.getenv("OPENAI_MODEL", "gpt-4o-mini")


@asynccontextmanager
async def lifespan(app: FastAPI):
    print("[Startup] Inicializando e indexando base de conhecimento (RAG)...")
    get_vector_store()
    print("[Startup] RAG carregado com sucesso.")

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


@app.get("/health")
async def health_check():
    """Liveness/readiness probe para o agente."""
    return {"status": "ok"}


@app.get("/metrics")
async def prometheus_metrics():
    """Endpoint de métricas Prometheus."""
    body, content_type = metrics_response()
    return Response(content=body, media_type=content_type)


@app.post("/generate")
async def generate_response(body: ContextRequest, request: Request):
    """
    Endpoint principal do assistente bancário.

    Recebe: {customer_id: str, query: str}
    Retorna: resposta contextualizada com justificativa estruturada.

    O supervisor (LangGraph) decide quais sub-agentes acionar
    e chama o BFA via MCP para obter dados de domínio.
    """
    start = time.time()

    try:
        rate_limiter.check(body.customer_id)
    except RateLimitError as e:
        REQUESTS_TOTAL.labels(status="rate_limited").inc()
        raise HTTPException(status_code=e.status_code, detail=e.message)

    try:
        validate_input_length(body.query)
    except InputValidationError as e:
        REQUESTS_TOTAL.labels(status="invalid_input").inc()
        raise HTTPException(status_code=e.status_code, detail=e.message)

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

        input_tokens = 0
        output_tokens = 0
        if langfuse_handler:
            langfuse_handler.flush()
            try:
                trace = langfuse_handler.trace
                if trace and hasattr(trace, "output"):
                    pass
            except Exception:
                pass

        if langfuse_handler and hasattr(langfuse_handler, "runs"):
            for run_data in getattr(langfuse_handler, "runs", {}).values():
                usage = getattr(run_data, "usage", None) or {}
                if isinstance(usage, dict):
                    input_tokens += usage.get("prompt_tokens", 0) or usage.get("input_tokens", 0)
                    output_tokens += usage.get("completion_tokens", 0) or usage.get("output_tokens", 0)

        if input_tokens == 0:
            input_tokens = max(1, len(body.query) // 4)
            context_parts = [
                result_state.get("profile_result", ""),
                result_state.get("transaction_result", ""),
                result_state.get("knowledge_result", ""),
            ]
            input_tokens += sum(len(p) // 4 for p in context_parts if p)
        if output_tokens == 0:
            final = result_state.get("final_answer", "")
            output_tokens = max(1, len(final) // 4)

        record_token_usage(OPENAI_MODEL, input_tokens, output_tokens)

        risk_score = result_state.get("risk_score", 0.0)
        if risk_score >= 1.0:
            GUARDRAIL_BLOCKS.labels(guardrail_type="input").inc()
        elif risk_score > 0.7:
            GUARDRAIL_BLOCKS.labels(guardrail_type="risk_classification").inc()

        final_answer = result_state.get("final_answer", "")
        if not final_answer:
            final_answer = (
                f"Justificativa: {result_state.get('justification', '')}. "
                f"Ferramentas utilizadas: {', '.join(result_state.get('tools_used', [])) or 'nenhuma'}"
            )

        final_answer = mask_pii(final_answer)

        latency = time.time() - start
        REQUEST_LATENCY.observe(latency)
        REQUESTS_TOTAL.labels(status="success").inc()

        return {
            "customer_id": body.customer_id,
            "query": body.query,
            "response": final_answer,
            "metadata": {
                "justification": mask_pii(result_state.get("justification", "")),
                "tools_used": result_state.get("tools_used", []),
                "plan": result_state.get("plan", ""),
                "risk_score": result_state.get("risk_score", 0.0),
                "agents_called": result_state.get("agents_to_call", []),
                "tokens": {
                    "input": input_tokens,
                    "output": output_tokens,
                },
                "latency_seconds": round(latency, 3),
            },
        }
    except HTTPException:
        raise
    except Exception as e:
        latency = time.time() - start
        REQUEST_LATENCY.observe(latency)
        REQUESTS_TOTAL.labels(status="error").inc()
        raise HTTPException(status_code=500, detail=str(e))
