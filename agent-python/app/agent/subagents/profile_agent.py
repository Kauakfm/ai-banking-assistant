"""
Agente de Perfil — Sub-agente especializado em dados cadastrais.

Responsabilidade: consultar o perfil do cliente via BFA (MCP)
e retornar um resumo dos dados cadastrais.

Fluxo: LLM → tool_call(get_customer_profile) → MCP → BFA Go → Profile API
"""

import os
from langchain_openai import ChatOpenAI
from langchain_core.messages import SystemMessage, HumanMessage, ToolMessage
from langchain_core.runnables import RunnableConfig
from app.agent.state import AgentState
from app.agent.prompts.profile_agent import PROFILE_AGENT_PROMPT

OPENAI_MODEL = os.getenv("OPENAI_MODEL", "gpt-4o-mini")
MAX_ITERATIONS = 3


def create_profile_agent(mcp_tools: list):
    """
    Factory que cria o nó do agente de perfil com as ferramentas MCP injetadas.
    As ferramentas são carregadas via MCP do BFA server.
    """
    profile_tools = [t for t in mcp_tools if t.name == "get_customer_profile"]

    async def profile_agent_node(state: AgentState, config: RunnableConfig = None) -> dict:
        llm = ChatOpenAI(model=OPENAI_MODEL, temperature=0).bind_tools(profile_tools)

        messages = [
            SystemMessage(content=PROFILE_AGENT_PROMPT),
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

                tool_map = {t.name: t for t in profile_tools}
                for tc in response.tool_calls:
                    if tc["name"] in tool_map:
                        try:
                            result = await tool_map[tc["name"]].ainvoke(tc["args"])
                        except Exception as e:
                            result = f"Erro ao executar {tc['name']}: {str(e)}"
                        messages.append(
                            ToolMessage(content=str(result), tool_call_id=tc["id"])
                        )
                        if tc["name"] not in tools_used:
                            tools_used.append(tc["name"])
                        tool_count += 1

            result_content = response.content if response else "Sem resposta do agente de perfil."

        except Exception as e:
            result_content = f"Erro no agente de perfil: {str(e)}"

        return {
            "profile_result": result_content,
            "tools_used": tools_used,
            "tool_call_count": tool_count,
            "messages": [m for m in messages if not isinstance(m, SystemMessage)],
        }

    return profile_agent_node
