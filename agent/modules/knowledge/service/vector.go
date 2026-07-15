package service

import "github.com/pgvector/pgvector-go"

// StorageVectorDimension 表示最终态 knowledge_vectors.embedding 的固定维度。
const StorageVectorDimension = 1536

// normalizeVector 将外部模型向量统一到 PostgreSQL knowledge_vectors 的固定维度。
//
// 简要描述：不足维度时在尾部补零，超过维度时截断；同一原始维度的向量统一补零
// 不改变余弦相似度，同时保证写入和查询遵守完全相同的 pgvector 维度契约。
func normalizeVector(input []float32, dimension int) pgvector.Vector {
	if dimension <= 0 {
		dimension = StorageVectorDimension
	}
	output := make([]float32, dimension)
	copy(output, input)
	return pgvector.NewVector(output)
}
