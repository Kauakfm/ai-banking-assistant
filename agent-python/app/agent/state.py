from typing import TypedDict, List, Annotated
from langchain_core.messages import BaseMessage
from langgraph.graph.message import add_messages

class AgentState(TypedDict):
    messages: Annotated[List[BaseMessage], add_messages]
    customer_id: str
    query: str
    financial_context: dict
    plan: str
    tools_used: List[str]
    final_answer: str
    justification: str