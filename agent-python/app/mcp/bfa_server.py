"""
MCP Server — BFA Banking

Servidor MCP que expõe as operações do BFA Go como ferramentas MCP.
Os agentes se conectam a este servidor via protocolo MCP (stdio)
para acessar dados de domínio com cache, resiliência e políticas
aplicadas automaticamente pelo BFA Go.

Fluxo: Agente → MCP Protocol → Este Server → BFA Go REST → APIs de Domínio
"""

import os
import json
import httpx
from mcp.server.fastmcp import FastMCP

BFA_GO_URL = os.getenv("BFA_GO_URL", "http://bfa-go:8080")

mcp = FastMCP("bfa-banking")


@mcp.tool()
async def get_customer_profile(customer_id: str) -> str:
    """
    Consulta o perfil cadastral do cliente no sistema bancário via BFA.
    Retorna dados como nome, e-mail, segmento e conta.
    O BFA aplica cache, resiliência e políticas automaticamente.
    Use quando precisar de informações cadastrais do cliente.
    """
    url = f"{BFA_GO_URL}/v1/customers/{customer_id}/profile"
    try:
        async with httpx.AsyncClient() as client:
            response = await client.get(url, timeout=10.0)
            response.raise_for_status()
            return json.dumps(response.json(), ensure_ascii=False, default=str)
    except httpx.HTTPStatusError as e:
        return f"Erro ao consultar perfil do cliente: HTTP {e.response.status_code}"
    except Exception as e:
        return f"Erro ao consultar perfil do cliente: {str(e)}"


@mcp.tool()
async def get_customer_transactions(customer_id: str) -> str:
    """
    Consulta as transações recentes do cliente no sistema bancário via BFA.
    Retorna lista de transações com data, descrição, valor, categoria e tipo (credit/debit).
    O BFA aplica cache, resiliência e políticas automaticamente.
    Use quando precisar analisar movimentações financeiras do cliente.
    """
    url = f"{BFA_GO_URL}/v1/customers/{customer_id}/transactions"
    try:
        async with httpx.AsyncClient() as client:
            response = await client.get(url, timeout=10.0)
            response.raise_for_status()
            return json.dumps(response.json(), ensure_ascii=False, default=str)
    except httpx.HTTPStatusError as e:
        return f"Erro ao consultar transações do cliente: HTTP {e.response.status_code}"
    except Exception as e:
        return f"Erro ao consultar transações do cliente: {str(e)}"


if __name__ == "__main__":
    mcp.run(transport="stdio")
