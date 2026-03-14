import os
from langchain_community.document_loaders import TextLoader
from langchain_text_splitters import RecursiveCharacterTextSplitter
from langchain_openai import OpenAIEmbeddings
from langchain_community.vectorstores import Chroma

CHROMA_PATH = "./chroma_db"
DATA_PATH = "./data/knowledge_base.txt"

def build_vector_store():
    if not os.path.exists(DATA_PATH):
        print(f"Arquivo não encontrado: {DATA_PATH}. O RAG não terá dados base.")
        return None
        
    loader = TextLoader(DATA_PATH, encoding="utf-8")
    documents = loader.load()

    text_splitter = RecursiveCharacterTextSplitter(
        chunk_size=500,
        chunk_overlap=50,
        separators=["\n\n", "\n", " ", ""]
    )
    chunks = text_splitter.split_documents(documents)
    
    embeddings = OpenAIEmbeddings(model="text-embedding-3-small")

    vector_store = Chroma.from_documents(
        documents=chunks, 
        embedding=embeddings, 
        persist_directory=CHROMA_PATH
    )
    print(f"RAG Indexado com sucesso. Total de chunks: {len(chunks)}")
    return vector_store

def get_vector_store():
    embeddings = OpenAIEmbeddings(model="text-embedding-3-small")
    if os.path.exists(CHROMA_PATH):
        return Chroma(persist_directory=CHROMA_PATH, embedding_function=embeddings)
    return build_vector_store()