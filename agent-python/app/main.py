import os
from dotenv import load_dotenv
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from langchain_core.messages import HumanMessage
from app.agent.graph import agent_app

# Carrega variáveis de ambiente do arquivo .env (se existir)
# Útil para desenvolvimento local; em Docker, usa variáveis de ambiente diretos
try:
    load_dotenv()
except:
    pass

app = FastAPI(title="GenAI Agent API")

class ContextRequest(BaseModel):
    customer_id: str
    query: str
    financial_context: dict

@app.post("/generate")
async def generate_response(request: ContextRequest):
    try:
        initial_state = {
            "customer_id": request.customer_id,
            "query": request.query,
            "financial_context": request.financial_context,
            "messages": [HumanMessage(content=request.query)],
            "tools_used": []
        }
        
        result_state = agent_app.invoke(initial_state)
        
        return {
            "answer": result_state.get("final_answer", ""),
            "justification": result_state.get("justification", ""),
            "tools_used": result_state.get("tools_used", []),
            "plan_generated": result_state.get("plan", "")
        }
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))