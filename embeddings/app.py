#!/usr/bin/env python3
"""
letheClaw Embedding Service
Generates 384-dimensional vector embeddings for semantic search.

This is NOT a full LLM - it only converts text to vectors using a small
pre-trained model (all-MiniLM-L6-v2, 80MB). CPU-only, no GPU required.
"""

import os
import logging
from flask import Flask, request, jsonify
from sentence_transformers import SentenceTransformer

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

# Initialize Flask app
app = Flask(__name__)

# Configuration
MODEL_NAME = os.getenv('MODEL', 'sentence-transformers/all-MiniLM-L6-v2')
DEVICE = os.getenv('DEVICE', 'cpu')

# Load model at startup
logger.info(f"Loading embedding model: {MODEL_NAME} on device: {DEVICE}")
model = SentenceTransformer(MODEL_NAME, device=DEVICE)
logger.info(f"Model loaded successfully. Embedding dimension: {model.get_sentence_embedding_dimension()}")


@app.route('/health', methods=['GET'])
def health():
    """Health check endpoint"""
    return jsonify({
        'status': 'healthy',
        'service': 'letheClaw Embeddings',
        'model': MODEL_NAME,
        'device': DEVICE,
        'dimension': model.get_sentence_embedding_dimension()
    })


@app.route('/embed', methods=['POST'])
def embed():
    """
    Generate embedding for input text
    
    Request body:
    {
        "text": "Your text here"
    }
    
    Response:
    {
        "embedding": [0.234, -0.156, ...],
        "dimension": 384
    }
    """
    try:
        data = request.get_json()
        
        if not data or 'text' not in data:
            return jsonify({'error': 'Missing "text" field in request body'}), 400
        
        text = data['text']
        
        if not text or not isinstance(text, str):
            return jsonify({'error': '"text" must be a non-empty string'}), 400
        
        # Generate embedding
        embedding = model.encode(text, convert_to_tensor=False)
        embedding_list = embedding.tolist()
        
        return jsonify({
            'embedding': embedding_list,
            'dimension': len(embedding_list)
        })
    
    except Exception as e:
        logger.error(f"Error generating embedding: {str(e)}", exc_info=True)
        return jsonify({'error': f'Failed to generate embedding: {str(e)}'}), 500


@app.route('/embed/batch', methods=['POST'])
def embed_batch():
    """
    Generate embeddings for multiple texts
    
    Request body:
    {
        "texts": ["Text 1", "Text 2", ...]
    }
    
    Response:
    {
        "embeddings": [[0.234, ...], [0.156, ...], ...],
        "dimension": 384,
        "count": 2
    }
    """
    try:
        data = request.get_json()
        
        if not data or 'texts' not in data:
            return jsonify({'error': 'Missing "texts" field in request body'}), 400
        
        texts = data['texts']
        
        if not isinstance(texts, list) or not texts:
            return jsonify({'error': '"texts" must be a non-empty list'}), 400
        
        # Generate embeddings
        embeddings = model.encode(texts, convert_to_tensor=False)
        embeddings_list = [emb.tolist() for emb in embeddings]
        
        return jsonify({
            'embeddings': embeddings_list,
            'dimension': len(embeddings_list[0]),
            'count': len(embeddings_list)
        })
    
    except Exception as e:
        logger.error(f"Error generating batch embeddings: {str(e)}", exc_info=True)
        return jsonify({'error': f'Failed to generate embeddings: {str(e)}'}), 500


if __name__ == '__main__':
    port = int(os.getenv('PORT', 5000))
    logger.info(f"Starting embedding service on port {port}")
    app.run(host='0.0.0.0', port=port, debug=False)
