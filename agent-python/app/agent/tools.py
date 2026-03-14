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

@tool
def analyze_transactions(customer_id: str, transactions: list) -> str:
    """
    Analisa as transações recentes do cliente para identificar padrões de gastos ou recebimentos.
    """
    if not transactions:
        return "Nenhuma transação recente encontrada."
        
    total_recebido = sum(t.get('amount', 0) for t in transactions if t.get('type') == 'credit')
    return f"O cliente {customer_id} possui movimentação ativa. Total de recebimentos recentes: R$ {total_recebido:.2f}."

agent_tools = [rag_search, analyze_transactions]