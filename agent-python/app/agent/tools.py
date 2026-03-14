from langchain_core.tools import tool

@tool
def rag_search(query: str) -> str:
    """
    Busca na base de conhecimento (políticas de crédito, FAQs) informações relevantes para a dúvida do cliente.
    """
    # Mock inicial: Aqui entrará o ChromaDB na Parte 3
    return "Política Mock: Clientes com faturamento anual superior a R$ 500k têm limite pré-aprovado de 10% do faturamento."

@tool
def analyze_transactions(customer_id: str, transactions: list) -> str:
    """
    Analisa as transações recentes do cliente para identificar padrões de gastos ou recebimentos.
    """
    if not transactions:
        return "Nenhuma transação recente encontrada."
    return f"O cliente {customer_id} possui {len(transactions)} transações recentes. Movimentação ativa."

# Lista de ferramentas disponíveis para o executor
agent_tools = [rag_search, analyze_transactions]