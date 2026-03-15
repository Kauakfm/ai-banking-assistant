"""
Supervisor — Grafo principal do assistente bancário (Padrão BFA).

Implementa o pattern multi-agente com supervisor usando LangGraph:

┌──────────────────────────────────────────────────────────────┐
│  Cliente → Supervisor (LangGraph)                            │
│    ├── Input Guardrail (segurança)                           │
│    ├── Planner (decide quais agentes acionar)                │
│    ├── Routing condicional:                                  │
│    │   ├── Profile Agent → BFA (MCP) → Dados cadastrais     │
│    │   ├── Transaction Agent → BFA (MCP) → Transações       │
│    │   └── Knowledge Agent → RAG (local) → Base de conhec.  │
│    ├── Formatter (consolida respostas dos subagentes)        │
│    └── Output Guardrail (segurança)                          │
└──────────────────────────────────────────────────────────────┘
"""

import os
import json
from langchain_openai import ChatOpenAI
from langchain_core.messages import SystemMessage, HumanMessage
from langchain_core.runnables import RunnableConfig
from langgraph.graph import StateGraph, END

from app.agent.state import AgentState
from app.agent.guardrails import (
    sanitize_input,
    classify_risk,
    verify_output,
)
from app.agent.prompts.planner import PLANNER_SYSTEM_PROMPT
from app.agent.prompts.formatter import FORMATTER_SYSTEM_PROMPT
from app.agent.subagents.profile_agent import create_profile_agent
from app.agent.subagents.transaction_agent import create_transaction_agent
from app.agent.subagents.knowledge_agent import create_knowledge_agent

OPENAI_MODEL = os.getenv("OPENAI_MODEL", "gpt-4o-mini")


def build_supervisor(mcp_tools: list, rag_tool):
    """
    Constrói o grafo do supervisor com as ferramentas MCP e RAG injetadas.
    Chamado uma vez no startup da aplicação.
    """

    llm = ChatOpenAI(model=OPENAI_MODEL, temperature=0)

    # --- Sub-agentes especializados (cada um com suas ferramentas) ---
    profile_agent_node = create_profile_agent(mcp_tools)
    transaction_agent_node = create_transaction_agent(mcp_tools)
    knowledge_agent_node = create_knowledge_agent(rag_tool)

    # ================================================================
    #                         NÓS DO GRAFO
    # ================================================================

    def input_guardrail_node(state: AgentState, config: RunnableConfig = None) -> dict:
        """Guardrail de entrada: sanitização + classificação de risco."""
        query = state["query"]

        sanitized, injection_detected = sanitize_input(query)
        if injection_detected:
            return {
                "final_answer": "Operação bloqueada pelas políticas de segurança do banco.",
                "justification": "Alerta de Segurança: Padrão de injeção de prompt detectado na entrada.",
                "risk_score": 1.0,
            }

        risk = classify_risk(sanitized, config=config)
        risk_score = risk.get("risk_score", 0.0)

        if risk_score > 0.7:
            return {
                "final_answer": "Solicitação bloqueada por políticas de segurança.",
                "justification": f"Risco detectado: {risk.get('risk_type', 'UNKNOWN')} (score: {risk_score})",
                "risk_score": risk_score,
            }

        return {"risk_score": risk_score}

    async def planner_node(state: AgentState, config: RunnableConfig = None) -> dict:
        """
        Planner: analisa a dúvida e decide quais sub-agentes acionar.
        Retorna o plano de execução + lista de agentes necessários.
        """
        sys_prompt = SystemMessage(content=PLANNER_SYSTEM_PROMPT)
        user_prompt = HumanMessage(
            content=f"ID do cliente: {state['customer_id']}\nDúvida: {state['query']}"
        )

        json_llm = llm.bind(response_format={"type": "json_object"})
        response = await json_llm.ainvoke([sys_prompt, user_prompt], config=config)

        try:
            parsed = json.loads(response.content)
            agents = parsed.get("agents", ["profile", "transactions", "knowledge"])
            plan = parsed.get("plan", response.content)
        except (json.JSONDecodeError, AttributeError):
            agents = ["profile", "transactions", "knowledge"]
            plan = response.content if hasattr(response, "content") else "Plano não estruturado."

        return {"plan": plan, "agents_to_call": agents}

    async def formatter_node(state: AgentState, config: RunnableConfig = None) -> dict:
        """
        Formatter: consolida respostas de todos os sub-agentes
        em uma resposta final estruturada com justificativa.
        """
        context_parts = []
        if state.get("profile_result"):
            context_parts.append(
                f"[Dados do Perfil do Cliente]\n{state['profile_result']}"
            )
        if state.get("transaction_result"):
            context_parts.append(
                f"[Dados de Transações]\n{state['transaction_result']}"
            )
        if state.get("knowledge_result"):
            context_parts.append(
                f"[Informações da Base de Conhecimento]\n{state['knowledge_result']}"
            )

        context = "\n\n".join(context_parts) if context_parts else "Nenhum dado coletado."

        messages = [
            SystemMessage(content=FORMATTER_SYSTEM_PROMPT),
            HumanMessage(
                content=(
                    f"Dúvida original do cliente: {state['query']}\n\n"
                    f"Plano de execução: {state.get('plan', 'N/A')}\n\n"
                    f"Informações coletadas pelos agentes especializados:\n{context}"
                )
            ),
        ]

        final_llm = ChatOpenAI(model=OPENAI_MODEL, temperature=0).bind(
            response_format={"type": "json_object"}
        )
        response = await final_llm.ainvoke(messages, config=config)

        try:
            parsed = json.loads(response.content)
            return {
                "final_answer": parsed.get("answer", ""),
                "justification": parsed.get("justification", ""),
            }
        except (json.JSONDecodeError, AttributeError):
            return {
                "final_answer": response.content if hasattr(response, "content") else "",
                "justification": "Erro ao estruturar justificativa.",
            }

    def output_guardrail_node(state: AgentState, config: RunnableConfig = None) -> dict:
        """Guardrail de saída: verifica se a resposta é segura e fundamentada."""
        answer = state.get("final_answer", "")
        if not answer:
            return {}

        context_parts = []
        if state.get("profile_result"):
            context_parts.append(state["profile_result"])
        if state.get("transaction_result"):
            context_parts.append(state["transaction_result"])
        if state.get("knowledge_result"):
            context_parts.append(state["knowledge_result"])

        context = "\n".join(context_parts)
        review = verify_output(answer, context, config=config)

        if not review.get("approved", True):
            return {
                "final_answer": "Não foi possível gerar uma resposta segura neste momento.",
                "justification": f"Auditoria de saída: {review.get('reason', 'Resposta reprovada.')}",
            }

        return {}

    # ================================================================
    #                    FUNÇÕES DE ROTEAMENTO
    # ================================================================

    def check_input_safety(state: AgentState) -> str:
        """Decide se a entrada é segura para prosseguir."""
        if state.get("final_answer"):
            return "unsafe"
        return "safe"

    def route_after_plan(state: AgentState) -> str:
        """Rota condicional: planner → primeiro agente necessário."""
        agents = state.get("agents_to_call", [])
        if "profile" in agents:
            return "profile"
        if "transactions" in agents:
            return "transactions"
        if "knowledge" in agents:
            return "knowledge"
        return "formatter"

    def route_after_profile(state: AgentState) -> str:
        """Rota condicional: profile_agent → próximo agente ou formatter."""
        agents = state.get("agents_to_call", [])
        if "transactions" in agents:
            return "transactions"
        if "knowledge" in agents:
            return "knowledge"
        return "formatter"

    def route_after_transactions(state: AgentState) -> str:
        """Rota condicional: transaction_agent → knowledge_agent ou formatter."""
        agents = state.get("agents_to_call", [])
        if "knowledge" in agents:
            return "knowledge"
        return "formatter"

    # ================================================================
    #                   CONSTRUÇÃO DO GRAFO
    # ================================================================

    workflow = StateGraph(AgentState)

    # Nós
    workflow.add_node("input_guardrail", input_guardrail_node)
    workflow.add_node("planner", planner_node)
    workflow.add_node("profile_agent", profile_agent_node)
    workflow.add_node("transaction_agent", transaction_agent_node)
    workflow.add_node("knowledge_agent", knowledge_agent_node)
    workflow.add_node("formatter", formatter_node)
    workflow.add_node("output_guardrail", output_guardrail_node)

    # Ponto de entrada
    workflow.set_entry_point("input_guardrail")

    # Edges condicionais
    workflow.add_conditional_edges(
        "input_guardrail",
        check_input_safety,
        {"safe": "planner", "unsafe": END},
    )

    workflow.add_conditional_edges(
        "planner",
        route_after_plan,
        {
            "profile": "profile_agent",
            "transactions": "transaction_agent",
            "knowledge": "knowledge_agent",
            "formatter": "formatter",
        },
    )

    workflow.add_conditional_edges(
        "profile_agent",
        route_after_profile,
        {
            "transactions": "transaction_agent",
            "knowledge": "knowledge_agent",
            "formatter": "formatter",
        },
    )

    workflow.add_conditional_edges(
        "transaction_agent",
        route_after_transactions,
        {
            "knowledge": "knowledge_agent",
            "formatter": "formatter",
        },
    )

    workflow.add_edge("knowledge_agent", "formatter")
    workflow.add_edge("formatter", "output_guardrail")
    workflow.add_edge("output_guardrail", END)

    return workflow.compile()
