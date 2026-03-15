"""
Agente de Conhecimento — Sub-agente especializado em políticas e regras bancárias.

Responsabilidade: consultar a base de conhecimento via RAG (ChromaDB)
e retornar informações contextuais sobre políticas, FAQs e regras do banco.

Fluxo: LLM → tool_call(rag_search) → ChromaDB (local)
"""

import os
from langchain_openai import ChatOpenAI
from langchain_core.messages import SystemMessage, HumanMessage, ToolMessage
from langchain_core.runnables import RunnableConfig
from app.agent.state import AgentState
from app.agent.prompts.knowledge_agent import KNOWLEDGE_AGENT_PROMPT

OPENAI_MODEL = os.getenv("OPENAI_MODEL", "gpt-4o-mini")
MAX_ITERATIONS = 3


def create_knowledge_agent(rag_tool):
    """
    Factory que cria o nó do agente de conhecimento com a ferramenta RAG injetada.
    A ferramenta rag_search é local (ChromaDB), não passa pelo MCP/BFA.
    """
    knowledge_tools = [rag_tool]

    async def knowledge_agent_node(state: AgentState, config: RunnableConfig = None) -> dict:
        llm = ChatOpenAI(model=OPENAI_MODEL, temperature=0).bind_tools(knowledge_tools)

        messages = [
            SystemMessage(content=KNOWLEDGE_AGENT_PROMPT),
            HumanMessage(
                content=f"Dúvida do cliente: {state['query']}"
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

                tool_map = {t.name: t for t in knowledge_tools}
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

            result_content = response.content if response else "Sem resposta do agente de conhecimento."

        except Exception as e:
            result_content = f"Erro no agente de conhecimento: {str(e)}"

        return {
            "knowledge_result": result_content,
            "tools_used": tools_used,
            "tool_call_count": tool_count,
            "messages": [m for m in messages if not isinstance(m, SystemMessage)],
        }

    return knowledge_agent_node
