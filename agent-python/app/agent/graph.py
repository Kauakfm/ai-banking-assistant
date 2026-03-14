import json
from langchain_openai import ChatOpenAI
from langchain_core.messages import SystemMessage, HumanMessage, ToolMessage
from langgraph.graph import StateGraph, END
from app.agent.state import AgentState
from app.agent.tools import agent_tools

llm = ChatOpenAI(model="gpt-4o-mini", temperature=0)
llm_with_tools = llm.bind_tools(agent_tools)


def guardrail_node(state: AgentState):
    """Atua como um firewall. Verifica se a query contém tentativas de Prompt Injection."""
    query = state["query"].lower()
    
    # Lista básica de bloqueio (Em produção, usa-se modelos menores específicos de NLP para isso)
    malicious_keywords = [
        "ignore", "esqueça", "burlar", "desconsidere", 
        "instruções", "prompt", "system", "hack", "bypass"
    ]
    
    is_safe = True
    for word in malicious_keywords:
        if word in query:
            is_safe = False
            break
            
    if not is_safe:
        return {
            "final_answer": "Operação bloqueada pelas políticas de segurança do banco.",
            "justification": "Alerta de Segurança: Tentativa de manipulação de prompt detectada."
        }
    return {} # Passa ileso

def planner_node(state: AgentState):
    sys_prompt = SystemMessage(content="Você é um planejador financeiro B2B. Analise a dúvida do cliente e o contexto financeiro. Escreva um plano de no máximo 3 passos do que o agente executor deve buscar (ex: RAG, transações) para responder.")
    user_prompt = HumanMessage(content=f"Dúvida: {state['query']}\nContexto: {json.dumps(state['financial_context'])}")
    response = llm.invoke([sys_prompt, user_prompt])
    return {"plan": response.content}

def executor_node(state: AgentState):
    sys_prompt = SystemMessage(content=f"Você é o executor. Siga o plano: {state['plan']}. Use ferramentas se precisar. Baseie-se APENAS nas ferramentas.")
    messages = [sys_prompt] + state["messages"]
    response = llm_with_tools.invoke(messages)
    return {"messages": [response]}

def tool_node(state: AgentState):
    last_message = state["messages"][-1]
    tool_responses = []
    tools_used = state.get("tools_used", [])
    tool_map = {tool.name: tool for tool in agent_tools}
    
    for tool_call in last_message.tool_calls:
        tool_name = tool_call["name"]
        tool_args = tool_call["args"]
        if tool_name in tool_map:
            result = tool_map[tool_name].invoke(tool_args)
            tool_responses.append(ToolMessage(content=str(result), tool_call_id=tool_call["id"]))
            if tool_name not in tools_used:
                tools_used.append(tool_name)
                
    return {"messages": tool_responses, "tools_used": tools_used}

def formatter_node(state: AgentState):
    sys_prompt = SystemMessage(content="Você é um assistente sênior PJ. Com base nas informações coletadas, forneça uma resposta clara. Depois, forneça uma justificativa técnica da sua decisão baseada nos dados. Responda num formato JSON estruturado com chaves: 'answer' e 'justification'. NÃO invente taxas que não vieram do contexto.")
    messages = [sys_prompt] + state["messages"]
    final_llm = llm.bind(response_format={"type": "json_object"})
    response = final_llm.invoke(messages)
    
    try:
        parsed = json.loads(response.content)
        return {"final_answer": parsed.get("answer", ""), "justification": parsed.get("justification", "")}
    except:
        return {"final_answer": response.content, "justification": "Erro ao estruturar justificativa."}

def check_safety(state: AgentState):
    """Avalia o resultado do Guardrail."""
    if state.get("final_answer"): 
        return "unsafe"
    return "safe"

def should_continue(state: AgentState):
    last_message = state["messages"][-1]
    if last_message.tool_calls:
        return "tools"
    return "format"


workflow = StateGraph(AgentState)

workflow.add_node("guardrail", guardrail_node)
workflow.add_node("planner", planner_node)
workflow.add_node("executor", executor_node)
workflow.add_node("tools", tool_node)
workflow.add_node("formatter", formatter_node)

workflow.set_entry_point("guardrail")

workflow.add_conditional_edges("guardrail", check_safety, 
    {"safe": "planner", "unsafe": END}
)

workflow.add_edge("planner", "executor")
workflow.add_conditional_edges("executor", should_continue, {"tools": "tools", "format": "formatter"})
workflow.add_edge("tools", "executor")
workflow.add_edge("formatter", END)

agent_app = workflow.compile()