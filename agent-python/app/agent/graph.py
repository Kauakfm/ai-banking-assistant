import os
import json
from dotenv import load_dotenv
from langchain_openai import ChatOpenAI
from langchain_core.messages import SystemMessage, HumanMessage, ToolMessage
from langgraph.graph import StateGraph, END
from app.agent.state import AgentState
from app.agent.tools import agent_tools

# Carrega variáveis de ambiente do arquivo .env (se existir)
# Útil para desenvolvimento local; em Docker, usa variáveis de ambiente diretos
try:
    load_dotenv()
except:
    pass

llm = ChatOpenAI(model="gpt-4o-mini", temperature=0)
llm_with_tools = llm.bind_tools(agent_tools)


def planner_node(state: AgentState):
    """Analisa a pergunta e o contexto, e cria um plano de ação."""
    sys_prompt = SystemMessage(content="Você é um planejador financeiro B2B. Analise a dúvida do cliente e o contexto financeiro. Escreva um plano de no máximo 3 passos do que o agente executor deve buscar (ex: RAG, transações) para responder.")
    user_prompt = HumanMessage(content=f"Dúvida: {state['query']}\nContexto: {json.dumps(state['financial_context'])}")
    
    response = llm.invoke([sys_prompt, user_prompt])
    return {"plan": response.content}

def executor_node(state: AgentState):
    """Usa o plano para decidir se deve chamar ferramentas ou gerar a resposta final."""
    sys_prompt = SystemMessage(content=f"Você é o executor. Siga o plano: {state['plan']}. Use ferramentas se precisar.")
    
    # Passamos o histórico de mensagens para o LLM ter contexto contínuo
    messages = [sys_prompt] + state["messages"]
    
    response = llm_with_tools.invoke(messages)
    return {"messages": [response]}

def tool_node(state: AgentState):
    """Executa as ferramentas solicitadas pelo LLM."""
    last_message = state["messages"][-1]
    
    tool_responses = []
    tools_used = state.get("tools_used", [])
    
    # Mapeia as funções reais
    tool_map = {tool.name: tool for tool in agent_tools}
    
    for tool_call in last_message.tool_calls:
        tool_name = tool_call["name"]
        tool_args = tool_call["args"]
        
        # Executa a ferramenta
        if tool_name in tool_map:
            result = tool_map[tool_name].invoke(tool_args)
            tool_responses.append(ToolMessage(content=str(result), tool_call_id=tool_call["id"]))
            if tool_name not in tools_used:
                tools_used.append(tool_name)
                
    return {"messages": tool_responses, "tools_used": tools_used}

def formatter_node(state: AgentState):
    """Formata a resposta final conforme exigido pelo BFA."""
    sys_prompt = SystemMessage(content="Você é um assistente sênior PJ. Com base nas informações coletadas, forneça uma resposta clara. Depois, forneça uma justificativa técnica da sua decisão baseada nos dados. Responda num formato JSON estruturado com chaves: 'answer' e 'justification'.")
    
    messages = [sys_prompt] + state["messages"]
    
    # Forçamos a saída em JSON
    final_llm = llm.bind(response_format={"type": "json_object"})
    response = final_llm.invoke(messages)
    
    try:
        parsed = json.loads(response.content)
        return {"final_answer": parsed.get("answer", ""), "justification": parsed.get("justification", "")}
    except:
        return {"final_answer": response.content, "justification": "Erro ao estruturar justificativa."}

def should_continue(state: AgentState):
    """Decide se o executor chamou uma ferramenta ou se já terminou."""
    last_message = state["messages"][-1]
    if last_message.tool_calls:
        return "tools"
    return "format"

workflow = StateGraph(AgentState)

workflow.add_node("planner", planner_node)
workflow.add_node("executor", executor_node)
workflow.add_node("tools", tool_node)
workflow.add_node("formatter", formatter_node)

workflow.set_entry_point("planner")
workflow.add_edge("planner", "executor")
workflow.add_conditional_edges("executor", should_continue, {"tools": "tools", "format": "formatter"})
workflow.add_edge("tools", "executor")
workflow.add_edge("formatter", END)

agent_app = workflow.compile()