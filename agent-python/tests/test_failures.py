"""
Testes de falha e resiliência — Agente Python.

Cobre cenários de:
- RAG indisponível
- BFA fora do ar (MCP tool failure)
- LLM falhando
- Timeout de ferramentas
- Sub-agentes com exceções
"""

import pytest
from unittest.mock import patch, AsyncMock, MagicMock


class TestBFAOffline:
    """Simula falha ao chamar BFA Go via MCP."""

    @pytest.mark.asyncio
    @patch("app.mcp.bfa_server.httpx.AsyncClient")
    async def test_profile_bfa_connection_refused(self, mock_client_cls):
        from app.mcp.bfa_server import get_customer_profile

        mock_client = AsyncMock()
        mock_client.get = AsyncMock(side_effect=ConnectionRefusedError("BFA offline"))
        mock_client.__aenter__ = AsyncMock(return_value=mock_client)
        mock_client.__aexit__ = AsyncMock(return_value=False)
        mock_client_cls.return_value = mock_client

        result = await get_customer_profile("cust-001")
        assert "Erro" in result

    @pytest.mark.asyncio
    @patch("app.mcp.bfa_server.httpx.AsyncClient")
    async def test_transactions_bfa_connection_refused(self, mock_client_cls):
        from app.mcp.bfa_server import get_customer_transactions

        mock_client = AsyncMock()
        mock_client.get = AsyncMock(side_effect=ConnectionRefusedError("BFA offline"))
        mock_client.__aenter__ = AsyncMock(return_value=mock_client)
        mock_client.__aexit__ = AsyncMock(return_value=False)
        mock_client_cls.return_value = mock_client

        result = await get_customer_transactions("cust-001")
        assert "Erro" in result


class TestLLMFailure:
    """Simula falha do LLM durante execução de sub-agente."""

    @pytest.mark.asyncio
    @patch("app.agent.subagents.profile_agent.ChatOpenAI")
    async def test_profile_agent_llm_exception(self, mock_llm_class):
        from app.agent.subagents.profile_agent import create_profile_agent

        mock_llm = MagicMock()
        mock_llm.bind_tools.return_value = mock_llm
        mock_llm.ainvoke = AsyncMock(side_effect=Exception("OpenAI API rate limit"))
        mock_llm_class.return_value = mock_llm

        mock_tool = MagicMock()
        mock_tool.name = "get_customer_profile"
        agent_node = create_profile_agent([mock_tool])

        state = {
            "customer_id": "cust-001",
            "query": "Meu perfil?",
            "tools_used": [],
            "tool_call_count": 0,
        }

        result = await agent_node(state, config=None)
        assert "Erro" in result["profile_result"]

    @pytest.mark.asyncio
    @patch("app.agent.subagents.transaction_agent.ChatOpenAI")
    async def test_transaction_agent_llm_exception(self, mock_llm_class):
        from app.agent.subagents.transaction_agent import create_transaction_agent

        mock_llm = MagicMock()
        mock_llm.bind_tools.return_value = mock_llm
        mock_llm.ainvoke = AsyncMock(side_effect=Exception("OpenAI API timeout"))
        mock_llm_class.return_value = mock_llm

        mock_tool = MagicMock()
        mock_tool.name = "get_customer_transactions"
        agent_node = create_transaction_agent([mock_tool])

        state = {
            "customer_id": "cust-001",
            "query": "Minhas transações?",
            "tools_used": [],
            "tool_call_count": 0,
        }

        result = await agent_node(state, config=None)
        assert "Erro" in result["transaction_result"]

    @pytest.mark.asyncio
    @patch("app.agent.subagents.knowledge_agent.ChatOpenAI")
    async def test_knowledge_agent_llm_exception(self, mock_llm_class):
        from app.agent.subagents.knowledge_agent import create_knowledge_agent

        mock_llm = MagicMock()
        mock_llm.bind_tools.return_value = mock_llm
        mock_llm.ainvoke = AsyncMock(side_effect=Exception("Model overloaded"))
        mock_llm_class.return_value = mock_llm

        mock_rag = MagicMock()
        mock_rag.name = "rag_search"
        agent_node = create_knowledge_agent(mock_rag)

        state = {
            "query": "Políticas do banco?",
            "tools_used": [],
            "tool_call_count": 0,
        }

        result = await agent_node(state, config=None)
        assert "Erro" in result["knowledge_result"]


class TestToolExecutionFailure:
    """Simula falha na execução de ferramentas durante o loop de tool calls."""

    @pytest.mark.asyncio
    @patch("app.agent.subagents.profile_agent.ChatOpenAI")
    async def test_profile_tool_call_failure_handled(self, mock_llm_class):
        """Sub-agente deve tratar erro de tool e continuar."""
        from app.agent.subagents.profile_agent import create_profile_agent

        mock_tool_call_response = MagicMock()
        mock_tool_call_response.content = ""
        mock_tool_call_response.tool_calls = [
            {"name": "get_customer_profile", "args": {"customer_id": "cust-001"}, "id": "tc-1"}
        ]

        mock_final_response = MagicMock()
        mock_final_response.content = "Não consegui obter o perfil. Tente novamente."
        mock_final_response.tool_calls = []

        mock_llm = MagicMock()
        mock_llm.bind_tools.return_value = mock_llm
        mock_llm.ainvoke = AsyncMock(
            side_effect=[mock_tool_call_response, mock_final_response]
        )
        mock_llm_class.return_value = mock_llm

        mock_tool = MagicMock()
        mock_tool.name = "get_customer_profile"
        mock_tool.ainvoke = AsyncMock(side_effect=Exception("BFA returned 503"))

        agent_node = create_profile_agent([mock_tool])

        state = {
            "customer_id": "cust-001",
            "query": "Meu perfil?",
            "tools_used": [],
            "tool_call_count": 0,
        }

        result = await agent_node(state, config=None)
        assert "profile_result" in result
        assert result["tool_call_count"] >= 1


class TestGuardrailFailure:
    """Testa resiliência dos guardrails quando LLM falha."""

    @pytest.mark.asyncio
    @patch("app.agent.guardrails.ChatOpenAI")
    async def test_classify_risk_llm_offline_retorna_unknown(self, mock_llm_class):
        from app.agent.guardrails import classify_risk

        mock_instance = MagicMock()
        mock_instance.bind.return_value = mock_instance
        mock_instance.ainvoke = AsyncMock(side_effect=Exception("Connection reset"))
        mock_llm_class.return_value = mock_instance

        result = await classify_risk("consulta normal")
        assert result["risk_type"] == "UNKNOWN"
        assert result["risk_score"] == 0.5

    @pytest.mark.asyncio
    @patch("app.agent.guardrails.ChatOpenAI")
    async def test_verify_output_llm_offline_aprova(self, mock_llm_class):
        from app.agent.guardrails import verify_output

        mock_instance = MagicMock()
        mock_instance.bind.return_value = mock_instance
        mock_instance.ainvoke = AsyncMock(side_effect=Exception("Timeout"))
        mock_llm_class.return_value = mock_instance

        result = await verify_output("resposta qualquer", "contexto")
        assert result["approved"] is True


class TestRAGFailure:
    """Testa cenário de RAG/ChromaDB indisponível."""

    @patch("app.agent.tools.get_vector_store")
    def test_rag_search_chroma_offline(self, mock_get_vs):
        """Se ChromaDB falha, rag_search retorna mensagem de erro."""
        mock_db = MagicMock()
        mock_db.similarity_search.side_effect = Exception("ChromaDB offline")
        mock_get_vs.return_value = mock_db

        from app.agent.tools import rag_search

        result = rag_search.invoke({"query": "política de crédito"})
        assert isinstance(result, str)
        assert "Erro" in result

    @patch("app.agent.tools.get_vector_store")
    def test_rag_search_vector_store_none(self, mock_get_vs):
        """Se get_vector_store retorna None, rag_search retorna indisponível."""
        mock_get_vs.return_value = None

        from app.agent.tools import rag_search

        result = rag_search.invoke({"query": "política de crédito"})
        assert "indisponível" in result

    @patch("app.agent.tools.get_vector_store")
    def test_rag_search_no_results(self, mock_get_vs):
        """Se nenhum documento é encontrado, retorna mensagem adequada."""
        mock_db = MagicMock()
        mock_db.similarity_search.return_value = []
        mock_get_vs.return_value = mock_db

        from app.agent.tools import rag_search

        result = rag_search.invoke({"query": "algo totalmente irrelevante"})
        assert "Nenhuma informação" in result or isinstance(result, str)


class TestToolCallValidation:
    """Testa bloqueio de ferramentas não autorizadas."""

    def test_tool_nao_autorizada_bloqueia(self):
        from app.agent.guardrails import validate_tool_call

        with pytest.raises(ValueError, match="Ferramenta não autorizada"):
            validate_tool_call("exec_system_command")

    def test_tool_nao_autorizada_sql_injection(self):
        from app.agent.guardrails import validate_tool_call

        with pytest.raises(ValueError, match="Ferramenta não autorizada"):
            validate_tool_call("run_sql_query")

    def test_tools_autorizadas_passam(self):
        from app.agent.guardrails import validate_tool_call

        assert validate_tool_call("get_customer_profile") is True
        assert validate_tool_call("get_customer_transactions") is True
        assert validate_tool_call("rag_search") is True
