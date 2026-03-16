"""
Testes de integração — BFA Go ↔ Agent Python.

Cobre:
- MCP Server (bfa_server.py) chamando BFA Go REST
- Fluxo completo de sub-agente → MCP → BFA com mocks HTTP
"""

import pytest
import json
from unittest.mock import patch, AsyncMock, MagicMock


class TestMCPServerProfile:
    """Testa o tool get_customer_profile do MCP server."""

    @pytest.mark.asyncio
    @patch("app.mcp.bfa_server.httpx.AsyncClient")
    async def test_profile_sucesso(self, mock_client_cls):
        from app.mcp.bfa_server import get_customer_profile

        mock_response = MagicMock()
        mock_response.json.return_value = {
            "customer_id": "cust-001",
            "name": "João Silva",
            "email": "joao@email.com",
            "segment": "premium",
        }
        mock_response.raise_for_status = MagicMock()

        mock_client = AsyncMock()
        mock_client.get = AsyncMock(return_value=mock_response)
        mock_client.__aenter__ = AsyncMock(return_value=mock_client)
        mock_client.__aexit__ = AsyncMock(return_value=False)
        mock_client_cls.return_value = mock_client

        result = await get_customer_profile("cust-001")
        data = json.loads(result)
        assert data["customer_id"] == "cust-001"
        assert data["name"] == "João Silva"

    @pytest.mark.asyncio
    @patch("app.mcp.bfa_server.httpx.AsyncClient")
    async def test_profile_http_error(self, mock_client_cls):
        from app.mcp.bfa_server import get_customer_profile
        import httpx

        mock_response = MagicMock()
        mock_response.status_code = 404
        mock_response.raise_for_status.side_effect = httpx.HTTPStatusError(
            "Not Found", request=MagicMock(), response=mock_response
        )

        mock_client = AsyncMock()
        mock_client.get = AsyncMock(return_value=mock_response)
        mock_client.__aenter__ = AsyncMock(return_value=mock_client)
        mock_client.__aexit__ = AsyncMock(return_value=False)
        mock_client_cls.return_value = mock_client

        result = await get_customer_profile("cust-999")
        assert "Erro" in result
        assert "404" in result

    @pytest.mark.asyncio
    @patch("app.mcp.bfa_server.httpx.AsyncClient")
    async def test_profile_connection_error(self, mock_client_cls):
        from app.mcp.bfa_server import get_customer_profile

        mock_client = AsyncMock()
        mock_client.get = AsyncMock(side_effect=Exception("Connection refused"))
        mock_client.__aenter__ = AsyncMock(return_value=mock_client)
        mock_client.__aexit__ = AsyncMock(return_value=False)
        mock_client_cls.return_value = mock_client

        result = await get_customer_profile("cust-001")
        assert "Erro" in result
        assert "Connection refused" in result


class TestMCPServerTransactions:
    """Testa o tool get_customer_transactions do MCP server."""

    @pytest.mark.asyncio
    @patch("app.mcp.bfa_server.httpx.AsyncClient")
    async def test_transactions_sucesso(self, mock_client_cls):
        from app.mcp.bfa_server import get_customer_transactions

        mock_response = MagicMock()
        mock_response.json.return_value = {
            "customer_id": "cust-001",
            "transactions": [
                {
                    "date": "2025-01-15",
                    "description": "Supermercado",
                    "amount": -150.00,
                    "category": "groceries",
                    "type": "debit",
                },
                {
                    "date": "2025-01-14",
                    "description": "Salário",
                    "amount": 5000.00,
                    "category": "income",
                    "type": "credit",
                },
            ],
        }
        mock_response.raise_for_status = MagicMock()

        mock_client = AsyncMock()
        mock_client.get = AsyncMock(return_value=mock_response)
        mock_client.__aenter__ = AsyncMock(return_value=mock_client)
        mock_client.__aexit__ = AsyncMock(return_value=False)
        mock_client_cls.return_value = mock_client

        result = await get_customer_transactions("cust-001")
        data = json.loads(result)
        assert len(data["transactions"]) == 2
        assert data["transactions"][0]["category"] == "groceries"

    @pytest.mark.asyncio
    @patch("app.mcp.bfa_server.httpx.AsyncClient")
    async def test_transactions_http_500(self, mock_client_cls):
        from app.mcp.bfa_server import get_customer_transactions
        import httpx

        mock_response = MagicMock()
        mock_response.status_code = 500
        mock_response.raise_for_status.side_effect = httpx.HTTPStatusError(
            "Internal Server Error", request=MagicMock(), response=mock_response
        )

        mock_client = AsyncMock()
        mock_client.get = AsyncMock(return_value=mock_response)
        mock_client.__aenter__ = AsyncMock(return_value=mock_client)
        mock_client.__aexit__ = AsyncMock(return_value=False)
        mock_client_cls.return_value = mock_client

        result = await get_customer_transactions("cust-001")
        assert "Erro" in result
        assert "500" in result

    @pytest.mark.asyncio
    @patch("app.mcp.bfa_server.httpx.AsyncClient")
    async def test_transactions_timeout(self, mock_client_cls):
        from app.mcp.bfa_server import get_customer_transactions
        import httpx

        mock_client = AsyncMock()
        mock_client.get = AsyncMock(side_effect=httpx.ReadTimeout("Timeout"))
        mock_client.__aenter__ = AsyncMock(return_value=mock_client)
        mock_client.__aexit__ = AsyncMock(return_value=False)
        mock_client_cls.return_value = mock_client

        result = await get_customer_transactions("cust-001")
        assert "Erro" in result


class TestSubagentProfileIntegration:
    """Testa o sub-agente profile com ferramentas mockadas."""

    @pytest.mark.asyncio
    @patch("app.agent.subagents.profile_agent.ChatOpenAI")
    async def test_profile_agent_retorna_resultado(self, mock_llm_class):
        from app.agent.subagents.profile_agent import create_profile_agent

        mock_response = MagicMock()
        mock_response.content = "O cliente João Silva é premium."
        mock_response.tool_calls = []

        mock_llm = MagicMock()
        mock_llm.bind_tools.return_value = mock_llm
        mock_llm.ainvoke = AsyncMock(return_value=mock_response)
        mock_llm_class.return_value = mock_llm

        mock_tool = MagicMock()
        mock_tool.name = "get_customer_profile"
        agent_node = create_profile_agent([mock_tool])

        state = {
            "customer_id": "cust-001",
            "query": "Quem sou eu?",
            "tools_used": [],
            "tool_call_count": 0,
        }

        result = await agent_node(state, config=None)
        assert "profile_result" in result
        assert "João Silva" in result["profile_result"]


class TestSubagentTransactionIntegration:
    """Testa o sub-agente transaction com ferramentas mockadas."""

    @pytest.mark.asyncio
    @patch("app.agent.subagents.transaction_agent.ChatOpenAI")
    async def test_transaction_agent_retorna_resultado(self, mock_llm_class):
        from app.agent.subagents.transaction_agent import create_transaction_agent

        mock_response = MagicMock()
        mock_response.content = "2 transações: débito -R$150, crédito +R$5000."
        mock_response.tool_calls = []

        mock_llm = MagicMock()
        mock_llm.bind_tools.return_value = mock_llm
        mock_llm.ainvoke = AsyncMock(return_value=mock_response)
        mock_llm_class.return_value = mock_llm

        mock_tool = MagicMock()
        mock_tool.name = "get_customer_transactions"
        agent_node = create_transaction_agent([mock_tool])

        state = {
            "customer_id": "cust-001",
            "query": "Minhas transações recentes?",
            "tools_used": [],
            "tool_call_count": 0,
        }

        result = await agent_node(state, config=None)
        assert "transaction_result" in result
        assert "transações" in result["transaction_result"]


class TestSubagentKnowledgeIntegration:
    """Testa o sub-agente knowledge com ferramenta RAG mockada."""

    @pytest.mark.asyncio
    @patch("app.agent.subagents.knowledge_agent.ChatOpenAI")
    async def test_knowledge_agent_retorna_resultado(self, mock_llm_class):
        from app.agent.subagents.knowledge_agent import create_knowledge_agent

        mock_response = MagicMock()
        mock_response.content = "O limite do cartão pode ser ajustado via app."
        mock_response.tool_calls = []

        mock_llm = MagicMock()
        mock_llm.bind_tools.return_value = mock_llm
        mock_llm.ainvoke = AsyncMock(return_value=mock_response)
        mock_llm_class.return_value = mock_llm

        mock_rag = MagicMock()
        mock_rag.name = "rag_search"
        agent_node = create_knowledge_agent(mock_rag)

        state = {
            "query": "Como ajusto meu limite?",
            "tools_used": [],
            "tool_call_count": 0,
        }

        result = await agent_node(state, config=None)
        assert "knowledge_result" in result
        assert "limite" in result["knowledge_result"]
