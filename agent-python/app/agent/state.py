from typing import TypedDict, List, Annotated
from langchain_core.messages import BaseMessage
from langgraph.graph.message import add_messages


class AgentState(TypedDict):
    messages: Annotated[List[BaseMessage], add_messages]
    customer_id: str
    query: str
    plan: str
    tools_used: List[str]
    tool_call_count: int
    risk_score: float
    agents_to_call: List[str]
    profile_result: str
    transaction_result: str
    knowledge_result: str
    final_answer: str
    justification: str