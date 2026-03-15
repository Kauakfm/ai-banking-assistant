"""
Ferramentas locais do agente.

As ferramentas de domínio (get_customer_profile, get_customer_transactions)
agora são providas via MCP pelo BFA Server (app/mcp/bfa_server.py).
Aqui fica apenas a ferramenta RAG — busca local no ChromaDB.
"""

from langchain_core.tools import tool
from app.rag.indexer import get_vector_store


@tool
def rag_search(query: str) -> str:
    """
    Busca na base de conhecimento (políticas de crédito, FAQs) informações relevantes para a dúvida do cliente.
    Forneça uma query clara e objetiva para a busca semântica.
    """
    try:
        db = get_vector_store()
        if not db:
            return "Base de conhecimento indisponível no momento."

        docs = db.similarity_search(query, k=2)

        if not docs:
            return "Nenhuma informação relevante encontrada na base de conhecimento."

        context = "\n\n".join([doc.page_content for doc in docs])
        return f"Informação recuperada da base de conhecimento:\n{context}"

    except Exception as e:
        return f"Erro ao buscar na base de conhecimento: {str(e)}"