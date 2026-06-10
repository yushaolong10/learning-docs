
# LLM transformer

本文主要推演了transformer架构的矩阵变换，并讨论kvcache的原理细节。

## 1.Encoder

encoder的矩阵: [ i, love, NLP]

token: [batch_size, seq_len, emb_size] 

**定义w矩阵:**

wq:[emb_size, attention_size * multi_head]

wk:[emb_size, attention_size * multi_head]

wv:[emb_size, attention_size * multi_head]

**输出Q/K/V:**

q: [batch_size, seq_len, attention_size]

k: [batch_size, seq_len, attention_size]

v: [batch_size, seq_len, attention_size]

**由评分公式:**

softmax(Q*KT / 根号d + padding) * v
= [seq_len,seq_len] * [seq_len, attention_size]

**输出encoder结构:**

-> [batch_size, seq_len, attention_size]

-> ffn [batch_size, seq_len, emb_size]


## 2.Decoder


decoder的矩阵:[ 我, 爱, 自然，语言 处理]

decoder的过程，是通过causual mask进行掩码的，同一seq的，批token处理过程。

以`[ 我, 爱]`，推出`自然`为例，casual_mask如下:

```
(上三角矩阵)
[
  [0, -∞, -∞, -∞, -∞],
  [0,  0, -∞, -∞, -∞],
  [0,  0,  0, -∞, -∞],
  [0,  0,  0,  0, -∞],
  [0,  0,  0,  0,  0]
]
```

对'我爱'进行`embedding + position encoding` 

**当前参数值:**

target_seq_len=5 

causual_mask=[0,  0, -∞, -∞, -∞]

**当前token:**

token: [batch, target_seq_len, emb_size]

#### 每个decoder内部:

(1) 第一步:

**定义w矩阵:**

wq:[emb_size, attention_size * multi_head] + casual_mask

wk:[emb_size, attention_size * multi_head] + casual_mask

wv:[emb_size, attention_size * multi_head] + casual_mask

**输出Q/K/V:**

==> Q,K,V (思考推理阶段的 K,V cache) ==>

```
Q:[target_seq_len, attention_size] + casual_mask -> 推理时仅需要计算最后一行target_seq_len=2时的q
K:[target_seq_len, attention_size] + casual_mask -> 同上target_seq_len=2时的k
V:[target_seq_len, attention_size] + casual_mask -> 同上target_seq_len=2时的v		
```

(2) 第二步:

**定义w矩阵:**

使用交叉注意力机制 => 由 encoder 输出的 `[batch,src_seq_len, emb_size]`

wq:[emb_size, attention_size * multi_head] + casual_mask

#encoder不需要casual_mask

wk:[emb_size, attention_size * multi_head]

wv:[emb_size, attention_size * multi_head]

**注意：QKT=[target_seq_len, source_seq_len]**

==> 推出 QKT=[target_seq_len, source_seq_len]，非常重要，代表`目标`对`源`的所有关注程度

		             I     love    NLP
			<BOS>   0.1    0.2    0.1
			我      0.7    0.1    0.1
			爱      0.1    0.8    0.2

**输出Q/K/V:**

==>  Q,K,V (思考推理阶段的 K,V cache) ==>

	Q:[target_seq_len, attention_size] + casual_mask -> 推理时仅需要计算最后一行seq_len=2时的q
	K:[src_seq_len, emb_size, attention_size]        -> 同上seq_len=2时的k
	V:[src_seq_len, attention_size]                  -> 同上seq_len=2时的v


最后经过 ffn (相对于Q/K/V，计算参数量巨大)

hidden_size → vocab_size

经过 softmax，得到每个 token 的概率，模型用采样策略选择下一个 token。



## 3.KV cache

主要讨论 decoder-only 模型

比如已经生成: [我, 爱]

token: [batch_size, seq_len, emb_size]

**w矩阵值已固定:**

wq,wk,wv参数已经固定

**输出Q/K/V:**

==>  Q,K,V (推理阶段的 K,V cache) ==>

    Q: [seq_len, attention_size]   -> 推理时仅需要计算最后一行seq_len=2时的q
    K: [seq_len, attention_size]   -> 推理时仅需要计算最后一行seq_len=2时的k，进行缓存
    V: [seq_len, attention_size]   -> 推理时仅需要计算最后一行seq_len=2时的v，进行缓存

Q,K,V的值，随seq_len增大，会类似矩阵向下展开，之前已有的K、V计算的结果不变。

所以可以通过对K,V进行有效缓存，消耗一定的显存，提高GPU计算效率。