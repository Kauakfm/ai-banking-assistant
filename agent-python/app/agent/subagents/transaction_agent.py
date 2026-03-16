"""
Agente de Transações — Sub-agente especializado em movimentações financeiras.

Responsabilidade: consultar as transações do cliente via BFA (MCP)
e retornar um resumo das movimentações financeiras.

Fluxo: LLM → tool_call(get_customer_transactions) → MCP → BFA Go → Transaction API
"""

import os
from langchain_openai import ChatOpenAI
from langchain_core.messages import SystemMessage, HumanMessage, ToolMessage
from langchain_core.runnables import RunnableConfig
from app.agent.state import AgentState
from app.agent.guardrails import validate_tool_call
from app.agent.prompts.transaction_agent import TRANSACTION_AGENT_PROMPT
from app.metrics import TOOL_ERRORS

OPENAI_MODEL = os.getenv("OPENAI_MODEL", "gpt-4o-mini")
MAX_ITERATIONS = 3


def create_transaction_agent(mcp_tools: list):
    """
    Factory que cria o nó do agente de transações com as ferramentas MCP injetadas.
    As ferramentas são carregadas via MCP do BFA server.
    """
    transaction_tools = [t for t in mcp_tools if t.name == "get_customer_transactions"]

    async def transaction_agent_node(state: AgentState, config: RunnableConfig = None) -> dict:
        llm = ChatOpenAI(model=OPENAI_MODEL, temperature=0).bind_tools(transaction_tools)

        messages = [
            SystemMessage(content=TRANSACTION_AGENT_PROMPT),
            HumanMessage(
                content=f"Customer ID: {state['customer_id']}\nDúvida do cliente: {state['query']}"
            ),
        ]

        tools_used = list(state.get("tools_used", []))
        tool_count = state.get("tool_call_count", 0)

        try:
            response = None
            for _ in range(MAX_ITERATIONS):
                response = await llm.ainvoke(messages, config=config)
                messages.append(response)

                if not response.tool_calls:
                    break

                tool_map = {t.name: t for t in transaction_tools}
                for tc in response.tool_calls:
                    try:
                        validate_tool_call(tc["name"])
                    except ValueError as e:
                        TOOL_ERRORS.labels(tool_name=tc["name"]).inc()
                        result = f"Ferramenta bloqueada: {str(e)}"
                        messages.append(
                            ToolMessage(content=result, tool_call_id=tc["id"])
                        )
                        continue

                    if tc["name"] in tool_map:
                        try:
                            result = await tool_map[tc["name"]].ainvoke(tc["args"])
                        except Exception as e:
                            TOOL_ERRORS.labels(tool_name=tc["name"]).inc()
                            result = f"Erro ao executar {tc['name']}: {str(e)}"
                        messages.append(
                            ToolMessage(content=str(result), tool_call_id=tc["id"])
                        )
                        if tc["name"] not in tools_used:
                            tools_used.append(tc["name"])
                        tool_count += 1

            result_content = response.content if response else "Sem resposta do agente de transações."

        except Exception as e:
            result_content = f"Erro no agente de transações: {str(e)}"

        return {
            "transaction_result": result_content,
            "tools_used": tools_used,
            "tool_call_count": tool_count,
            "messages": [m for m in messages if not isinstance(m, SystemMessage)],
        }

    return transaction_agent_node
