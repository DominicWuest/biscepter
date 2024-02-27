Requirements:
- [poetry](https://python-poetry.org)
- [openapi-python-client](https://github.com/openapi-generators/openapi-python-client)

In this directory, run the following commands:
```bash
openapi-python-client generate --path ../../api/openapi.yml # Generate the openAPI python client
poetry install # Install client
# Run the biscepter webserver with the jobConf.yml in this directory
poetry run python main.py # Run the API client
```