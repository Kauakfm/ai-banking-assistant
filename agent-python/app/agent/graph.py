import os
import json
from langchain_openai import ChatOpenAI
from langchain_core.messages import SystemMessage, HumanMessage, ToolMessage
from langgraph.graph import StateGraph, END
from app.agent.state import AgentState
from app.agent.tools import agent_tools
from app.agent.guardrails import (
    sanitize_input,
    classify_risk,
    validate_tool_call,
    verify_output,
    MAX_TOOL_CALLS,
)
from app.agent.prompts.planner import PLANNER_SYSTEM_PROMPT
from app.agent.prompts.executor import EXECUTOR_SYSTEM_PROMPT
from app.agent.prompts.formatter import FORMATTER_SYSTEM_PROMPT

OPENAI_MODEL = os.getenv("OPENAI_MODEL", "gpt-4o-mini")

llm = ChatOpenAI(model=OPENAI_MODEL, temperature=0)
llm_with_tools = llm.bind_tools(agent_tools)


def input_guardrail_node(state: AgentState):
    query = state["query"]

    sanitized, injection_detected = sanitize_input(query)
    if injection_detected:
        return {
            "final_answer": "Operação bloqueada pelas políticas de segurança do banco.",
            "justification": "Alerta de Segurança: Padrão de injeção de prompt detectado na entrada.",
            "risk_score": 1.0,
        }

    risk = classify_risk(sanitized)
    risk_score = risk.get("risk_score", 0.0)

    if risk_score > 0.7:
        return {
            "final_answer": "Solicitação bloqueada por políticas de segurança.",
            "justification": f"Risco detectado: {risk.get('risk_type', 'UNKNOWN')} (score: {risk_score})",
            "risk_score": risk_score,
        }

    return {"risk_score": risk_score}


def planner_node(state: AgentState):
    sys_prompt = SystemMessage(content=PLANNER_SYSTEM_PROMPT)
    user_prompt = HumanMessage(content=f"ID do cliente: {state['customer_id']}\nDúvida: {state['query']}")
    response = llm.invoke([sys_prompt, user_prompt])
    return {"plan": response.content}


def executor_node(state: AgentState):
    sys_prompt = SystemMessage(content=EXECUTOR_SYSTEM_PROMPT.format(plan=state['plan']))
    messages = [sys_prompt] + state["messages"]
    response = llm_with_tools.invoke(messages)
    return {"messages": [response]}


def tool_node(state: AgentState):
    last_message = state["messages"][-1]
    tool_responses = []
    tools_used = state.get("tools_used", [])
    tool_call_count = state.get("tool_call_count", 0)
    tool_map = {tool.name: tool for tool in agent_tools}

    for tool_call in last_message.tool_calls:
        tool_name = tool_call["name"]
        tool_args = tool_call["args"]

        try:
            validate_tool_call(tool_name)
        except ValueError as e:
            tool_responses.append(
                ToolMessage(content=str(e), tool_call_id=tool_call["id"])
            )
            continue

        tool_call_count += 1
        if tool_call_count > MAX_TOOL_CALLS:
            tool_responses.append(
                ToolMessage(
                    content="Limite de chamadas de ferramentas excedido.",
                    tool_call_id=tool_call["id"],
                )
            )
            continue

        if tool_name in tool_map:
            result = tool_map[tool_name].invoke(tool_args)
            tool_responses.append(
                ToolMessage(content=str(result), tool_call_id=tool_call["id"])
            )
            if tool_name not in tools_used:
                tools_used.append(tool_name)

    return {"messages": tool_responses, "tools_used": tools_used, "tool_call_count": tool_call_count}


def formatter_node(state: AgentState):
    sys_prompt = SystemMessage(content=FORMATTER_SYSTEM_PROMPT)
    messages = [sys_prompt] + state["messages"]
    final_llm = ChatOpenAI(model=OPENAI_MODEL, temperature=0).bind(response_format={"type": "json_object"})
    response = final_llm.invoke(messages)

    try:
        parsed = json.loads(response.content)
        return {"final_answer": parsed.get("answer", ""), "justification": parsed.get("justification", "")}
    except Exception:
        return {"final_answer": response.content, "justification": "Erro ao estruturar justificativa."}


def output_guardrail_node(state: AgentState):
    answer = state.get("final_answer", "")
    if not answer:
        return {}

    context = "\n".join(
        msg.content for msg in state.get("messages", [])
        if hasattr(msg, "content") and msg.content
    )

    review = verify_output(answer, context)

    if not review.get("approved", True):
        return {
            "final_answer": "Não foi possível gerar uma resposta segura neste momento.",
            "justification": f"Auditoria de saída: {review.get('reason', 'Resposta reprovada.')}",
        }

    return {}


def check_input_safety(state: AgentState):
    if state.get("final_answer"):
        return "unsafe"
    return "safe"


def should_continue(state: AgentState):
    last_message = state["messages"][-1]
    if hasattr(last_message, "tool_calls") and last_message.tool_calls:
        return "tools"
    return "format"


workflow = StateGraph(AgentState)

workflow.add_node("input_guardrail", input_guardrail_node)
workflow.add_node("planner", planner_node)
workflow.add_node("executor", executor_node)
workflow.add_node("tools", tool_node)
workflow.add_node("formatter", formatter_node)
workflow.add_node("output_guardrail", output_guardrail_node)

workflow.set_entry_point("input_guardrail")

workflow.add_conditional_edges("input_guardrail", check_input_safety,
    {"safe": "planner", "unsafe": END}
)

workflow.add_edge("planner", "executor")
workflow.add_conditional_edges("executor", should_continue, {"tools": "tools", "format": "formatter"})
workflow.add_edge("tools", "executor")
workflow.add_edge("formatter", "output_guardrail")
workflow.add_edge("output_guardrail", END)

agent_app = workflow.compile()