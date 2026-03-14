import os
from contextlib import asynccontextmanager
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from langchain_core.messages import HumanMessage
from langfuse.callback import CallbackHandler # <-- NOVA IMPORTAÇÃO

from app.agent.graph import agent_app
from app.rag.indexer import get_vector_store

@asynccontextmanager
async def lifespan(app: FastAPI):
    print("Inicializando e indexando a base de conhecimento (RAG)...")
    get_vector_store()
    print("RAG carregado com sucesso. Servidor pronto.")
    yield
    print("Encerrando agente...")

app = FastAPI(title="GenAI Agent API", lifespan=lifespan)

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
        
        config = {}
        if os.getenv("LANGFUSE_PUBLIC_KEY") and os.getenv("LANGFUSE_SECRET_KEY"):
            langfuse_handler = CallbackHandler(
                secret_key=os.getenv("LANGFUSE_SECRET_KEY"),
                public_key=os.getenv("LANGFUSE_PUBLIC_KEY"),
                host=os.getenv("LANGFUSE_HOST", "https://cloud.langfuse.com")
            )
            config = {"callbacks": [langfuse_handler]}
                
        result_state = agent_app.invoke(initial_state, config=config)
        
        return {
            "answer": result_state.get("final_answer", ""),
            "justification": result_state.get("justification", ""),
            "tools_used": result_state.get("tools_used", []),
            "plan_generated": result_state.get("plan", "")
        }
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))