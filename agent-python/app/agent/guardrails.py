import os
import json
import re
from langchain_openai import ChatOpenAI
from langchain_core.messages import SystemMessage, HumanMessage
from langchain_core.runnables import RunnableConfig
from app.agent.prompts.risk_classifier import RISK_CLASSIFIER_SYSTEM_PROMPT
from app.agent.prompts.output_judge import OUTPUT_JUDGE_SYSTEM_PROMPT

OPENAI_MODEL = os.getenv("OPENAI_MODEL", "gpt-4o-mini")

MAX_TOOL_CALLS = 5

TOOL_POLICY = {
    "get_customer_profile": "financial_context",
    "get_customer_transactions": "financial_context",
    "rag_search": "knowledge_lookup",
}

INJECTION_PATTERNS = [
    r"ignor[ea]\s+.*?\s*instru[cç][õo]es",
    r"desconsiderr?\s+.*?\s*regras",
    r"revele?\s+.*?\s*prompt",
    r"finja\s+ser",
    r"aja?\s+como\s+.*?\s*sistema",
    r"burla[re]?\s+.*?\s*seguran[cç]a",
    r"mostre?\s+.*?\s*instruc",
    r"esque[cç][ea]\s+.*?\s*anteriores",
    r"ignore\s+.*?\s*instructions",
    r"disregard\s+.*?\s*system",
    r"reveal\s+.*?\s*prompt",
    r"act\s+as\s+.*?\s*system",
    r"bypass\s+.*?\s*security",
    r"forget\s+.*?\s*rules",
    r"pretend\s+you\s+are",
]


def sanitize_input(text: str) -> tuple[str, bool]:
    injection_detected = False
    sanitized = text
    for pattern in INJECTION_PATTERNS:
        if re.search(pattern, sanitized, flags=re.IGNORECASE):
            injection_detected = True
            sanitized = re.sub(pattern, "", sanitized, flags=re.IGNORECASE)
    sanitized = re.sub(r"\s+", " ", sanitized).strip()
    return sanitized, injection_detected


def classify_risk(query: str, config: RunnableConfig = None) -> dict:
    try:
        json_llm = ChatOpenAI(model=OPENAI_MODEL, temperature=0).bind(response_format={"type": "json_object"})
        response = json_llm.invoke(
            [SystemMessage(content=RISK_CLASSIFIER_SYSTEM_PROMPT), HumanMessage(content=query)],
            config=config,
        )
        return json.loads(response.content)
    except Exception:
        return {"risk_type": "UNKNOWN", "risk_score": 0.5}


def validate_tool_call(tool_name: str) -> bool:
    if tool_name not in TOOL_POLICY:
        raise ValueError(f"Ferramenta não autorizada: {tool_name}")
    return True


def verify_output(answer: str, context: str, config: RunnableConfig = None) -> dict:
    try:
        json_llm = ChatOpenAI(model=OPENAI_MODEL, temperature=0).bind(response_format={"type": "json_object"})
        response = json_llm.invoke(
            [
                SystemMessage(content=OUTPUT_JUDGE_SYSTEM_PROMPT),
                HumanMessage(
                    content=f"Resposta do assistente:\n{answer}\n\nContexto das ferramentas:\n{context}"
                ),
            ],
            config=config,
        )
        return json.loads(response.content)
    except Exception:
        return {"approved": True, "reason": "Falha na auditoria, aprovado por padrão."}
