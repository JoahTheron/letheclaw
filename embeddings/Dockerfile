FROM python:3.11-slim

WORKDIR /app

# Install dependencies (CPU-only PyTorch - no CUDA/GPU bloat)
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copy application
COPY app.py .

# Make app executable
RUN chmod +x app.py

# Expose port
EXPOSE 5000

# Health check (simple curl check)
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
  CMD python -c "import urllib.request; urllib.request.urlopen('http://localhost:5000/health')" || exit 1

# Run
CMD ["python", "app.py"]
